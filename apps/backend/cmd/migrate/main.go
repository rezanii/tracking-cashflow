// Command migrate applies or rolls back the schema. Shipping the runner as a Go binary
// means a deployment needs no extra CLI installed.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlserver"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/gorm"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/config"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/repository"
)

var databaseNamePattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func main() {
	command := flag.String("command", "up", "up, down, drop, version or force")
	steps := flag.Int("steps", 0, "number of steps for up or down; 0 means all for up and one for down")
	version := flag.Int("version", 0, "target version for force")
	path := flag.String("path", "migrations", "directory holding the migration files")
	flag.Parse()

	if err := run(*command, *steps, *version, *path); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
}

func run(command string, steps, version int, path string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	// A fresh SQL Server has no application database yet, and golang-migrate cannot create
	// the database it is asked to connect to.
	if err := ensureDatabase(cfg); err != nil {
		return err
	}

	db, err := repository.NewDatabase(cfg)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer closePool(db)

	pool, err := db.DB()
	if err != nil {
		return fmt.Errorf("access connection pool: %w", err)
	}

	driver, err := sqlserver.WithInstance(pool, &sqlserver.Config{DatabaseName: cfg.DBName})
	if err != nil {
		return fmt.Errorf("prepare migration driver: %w", err)
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve migration path: %w", err)
	}
	sourceURL := "file://" + filepath.ToSlash(absolute)

	migrator, err := migrate.NewWithDatabaseInstance(sourceURL, cfg.DBName, driver)
	if err != nil {
		return fmt.Errorf("open migrator: %w", err)
	}

	switch command {
	case "up":
		err = applyUp(migrator, steps)
	case "down":
		err = applyDown(migrator, steps)
	case "drop":
		if cfg.IsProduction() {
			return errors.New("drop is refused when APP_ENV is production")
		}
		err = migrator.Drop()
	case "version":
		return reportVersion(migrator)
	case "force":
		if version <= 0 {
			return errors.New("force requires a positive -version")
		}
		err = migrator.Force(version)
	default:
		return fmt.Errorf("unknown command %q", command)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		slog.Info("schema already up to date")
		return nil
	}
	if err != nil {
		return err
	}

	slog.Info("migration applied", "command", command)
	return reportVersion(migrator)
}

func applyUp(migrator *migrate.Migrate, steps int) error {
	if steps > 0 {
		return migrator.Steps(steps)
	}
	return migrator.Up()
}

// applyDown defaults to a single step. Rolling the whole schema back is destructive, so it
// has to be asked for explicitly with -steps.
func applyDown(migrator *migrate.Migrate, steps int) error {
	if steps > 0 {
		return migrator.Steps(-steps)
	}
	return migrator.Steps(-1)
}

func reportVersion(migrator *migrate.Migrate) error {
	current, dirty, err := migrator.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		slog.Info("no migration applied yet")
		return nil
	}
	if err != nil {
		return fmt.Errorf("read version: %w", err)
	}
	slog.Info("schema version", "version", current, "dirty", dirty)
	if dirty {
		return fmt.Errorf("schema version %d is dirty, fix it then run -command force -version %d", current, current)
	}
	return nil
}

// ensureDatabase connects to master and creates the application database when it is absent.
// CREATE DATABASE cannot be parameterised, so the name is validated against a strict pattern
// and bracket-quoted rather than interpolated raw.
func ensureDatabase(cfg config.Config) error {
	if !databaseNamePattern.MatchString(cfg.DBName) {
		return fmt.Errorf("database name %q may only contain letters, digits and underscores", cfg.DBName)
	}

	masterCfg := cfg
	masterCfg.DBName = "master"

	db, err := repository.NewDatabase(masterCfg)
	if err != nil {
		return fmt.Errorf("connect master: %w", err)
	}
	defer closePool(db)

	var exists int
	if err := db.Raw("SELECT COUNT(*) FROM sys.databases WHERE name = ?", cfg.DBName).Scan(&exists).Error; err != nil {
		return fmt.Errorf("check database exists: %w", err)
	}
	if exists > 0 {
		return nil
	}

	statement := fmt.Sprintf("CREATE DATABASE [%s]", cfg.DBName)
	if err := db.Exec(statement).Error; err != nil {
		return fmt.Errorf("create database %s: %w", cfg.DBName, err)
	}
	slog.Info("database created", "database", cfg.DBName)
	return nil
}

func closePool(db *gorm.DB) {
	pool, err := db.DB()
	if err != nil {
		return
	}
	_ = pool.Close()
}
