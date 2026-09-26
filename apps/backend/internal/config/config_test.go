package config

import (
	"strings"
	"testing"
)

// A single DATABASE_URL is what every host injects, so it has to win over the DB_* parts and
// fill in the fields the migrator and the logs still read.
func TestDatabaseURLTakesPrecedence(t *testing.T) {
	cfg := Config{
		DBHost:      "localhost",
		DBPort:      5432,
		DBUser:      "postgres",
		DBPassword:  "local",
		DBName:      "local_db",
		DBSSLMode:   "disable",
		DatabaseURL: "postgres://neondb_owner:secret@ep-x.ap-southeast-1.aws.neon.tech/neondb?sslmode=require",
	}
	if err := cfg.applyDatabaseURL(); err != nil {
		t.Fatalf("applyDatabaseURL returned error: %v", err)
	}

	if cfg.DBName != "neondb" {
		t.Fatalf("database = %q, want neondb", cfg.DBName)
	}
	if cfg.DBUser != "neondb_owner" || cfg.DBPassword != "secret" {
		t.Fatalf("credentials were not taken from the URL: %q / %q", cfg.DBUser, cfg.DBPassword)
	}
	if cfg.DBHost != "ep-x.ap-southeast-1.aws.neon.tech" {
		t.Fatalf("host = %q", cfg.DBHost)
	}
	if cfg.DBSSLMode != "require" {
		t.Fatalf("sslmode = %q, want require from the URL", cfg.DBSSLMode)
	}
	// The DSN is passed through untouched: rewriting it risks dropping a provider's options.
	if cfg.PostgresDSN() != cfg.DatabaseURL {
		t.Fatalf("DSN = %q, want the URL verbatim", cfg.PostgresDSN())
	}
	// A provider owns the database, so the migrator must not attempt CREATE DATABASE.
	if cfg.ManagesDatabase() {
		t.Fatal("ManagesDatabase should be false when a DATABASE_URL is set")
	}
}

func TestWithoutDatabaseURLThePartsBuildTheDSN(t *testing.T) {
	cfg := Config{
		DBHost:     "localhost",
		DBPort:     5432,
		DBUser:     "postgres",
		DBPassword: "p@ss word/#1",
		DBName:     "financial_management",
		DBSSLMode:  "disable",
	}
	dsn := cfg.PostgresDSN()

	// Reserved characters in the password must be escaped, or they would alter the connection
	// string instead of being part of the secret.
	if !strings.Contains(dsn, "p%40ss%20word%2F%231") {
		t.Fatalf("password was not escaped in %q", dsn)
	}
	if !strings.Contains(dsn, "/financial_management") || !strings.Contains(dsn, "sslmode=disable") {
		t.Fatalf("unexpected DSN: %q", dsn)
	}
	if !cfg.ManagesDatabase() {
		t.Fatal("ManagesDatabase should be true without a DATABASE_URL")
	}
	if !strings.Contains(cfg.MaintenanceDSN(), "/postgres?") {
		t.Fatalf("maintenance DSN should target the postgres database: %q", cfg.MaintenanceDSN())
	}
}

func TestDatabaseURLIsRejectedWhenUnusable(t *testing.T) {
	for name, raw := range map[string]string{
		"wrong scheme": "mysql://user:pass@host/db",
		"no database":  "postgres://user:pass@host",
		"not a url":    "postgres://user:pass@ho st/db",
	} {
		cfg := Config{DatabaseURL: raw}
		if err := cfg.applyDatabaseURL(); err == nil {
			t.Fatalf("%s was accepted: %q", name, raw)
		}
	}
}

// Production must not fall back to sending credentials in the clear.
func TestProductionRequiresTLS(t *testing.T) {
	base := Config{
		AppEnv: "production", DBUser: "u", DBPassword: "p", DBName: "d",
		JWTSecret: "0123456789012345678901234567890123",
		Telegram:  TelegramConfig{Mode: TelegramModeOff},
	}
	for _, mode := range []string{"disable", "allow"} {
		cfg := base
		cfg.DBSSLMode = mode
		if err := cfg.validate(); err == nil {
			t.Fatalf("production accepted DB_SSLMODE=%s", mode)
		}
	}
	cfg := base
	cfg.DBSSLMode = "require"
	if err := cfg.validate(); err != nil {
		t.Fatalf("production rejected require: %v", err)
	}
}
