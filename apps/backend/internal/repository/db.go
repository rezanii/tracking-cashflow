package repository

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/config"
)

// NewDatabase opens the SQL Server pool and verifies it before returning, so a bad
// configuration fails at startup rather than on the first request.
func NewDatabase(cfg config.Config) (*gorm.DB, error) {
	level := gormlogger.Warn
	if !cfg.IsProduction() {
		level = gormlogger.Info
	}

	db, err := gorm.Open(sqlserver.Open(cfg.SQLServerDSN()), &gorm.Config{
		Logger:                 gormlogger.Default.LogMode(level),
		SkipDefaultTransaction: true,
		NowFunc:                func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("open sql server: %w", err)
	}

	pool, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access connection pool: %w", err)
	}
	pool.SetMaxOpenConns(25)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(30 * time.Minute)

	if err := pool.Ping(); err != nil {
		return nil, fmt.Errorf("ping sql server: %w", err)
	}

	slog.Info("database connected", "host", cfg.DBHost, "database", cfg.DBName)
	return db, nil
}
