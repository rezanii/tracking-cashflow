package repository

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/config"
)

// NewDatabase opens the Postgres pool and verifies it before returning, so a bad
// configuration fails at startup rather than on the first request.
func NewDatabase(cfg config.Config) (*gorm.DB, error) {
	return open(cfg, cfg.PostgresDSN())
}

// NewMaintenanceDatabase connects to the "postgres" database so the migrator can create the
// application database.
func NewMaintenanceDatabase(cfg config.Config) (*gorm.DB, error) {
	return open(cfg, cfg.MaintenanceDSN())
}

func open(cfg config.Config, dsn string) (*gorm.DB, error) {
	level := gormlogger.Warn
	if !cfg.IsProduction() {
		level = gormlogger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                 gormlogger.Default.LogMode(level),
		SkipDefaultTransaction: true,
		NowFunc:                func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	pool, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access connection pool: %w", err)
	}
	pool.SetMaxOpenConns(25)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(30 * time.Minute)

	if err := pool.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	slog.Info("database connected", "host", cfg.DBHost, "database", cfg.DBName)
	return db, nil
}
