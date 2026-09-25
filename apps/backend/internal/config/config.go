package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv  string
	AppPort int

	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	JWTSecret     string
	JWTExpiration time.Duration

	CORSAllowedOrigins []string
}

func (c Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

// SQLServerDSN builds a TDS URL. Credentials are escaped so a password containing
// reserved characters cannot break or alter the connection string.
func (c Config) SQLServerDSN() string {
	query := url.Values{}
	query.Set("database", c.DBName)
	query.Set("encrypt", "disable")

	dsn := url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(c.DBUser, c.DBPassword),
		Host:     fmt.Sprintf("%s:%d", c.DBHost, c.DBPort),
		RawQuery: query.Encode(),
	}
	return dsn.String()
}

// Load reads the environment, applying .env when present. A missing .env is not an
// error: in Docker and CI the values arrive as real environment variables.
func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		AppEnv:             envString("APP_ENV", "development"),
		DBHost:             envString("DB_HOST", "localhost"),
		DBUser:             envString("DB_USER", ""),
		DBPassword:         envString("DB_PASSWORD", ""),
		DBName:             envString("DB_NAME", ""),
		JWTSecret:          envString("JWT_SECRET", ""),
		CORSAllowedOrigins: envStringSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
	}

	var err error
	if cfg.AppPort, err = envInt("APP_PORT", 8080); err != nil {
		return Config{}, err
	}
	if cfg.DBPort, err = envInt("DB_PORT", 1433); err != nil {
		return Config{}, err
	}
	if cfg.JWTExpiration, err = envDuration("JWT_EXPIRATION", 24*time.Hour); err != nil {
		return Config{}, err
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	var missing []string
	if c.DBUser == "" {
		missing = append(missing, "DB_USER")
	}
	if c.DBPassword == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if c.DBName == "" {
		missing = append(missing, "DB_NAME")
	}
	if c.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	if c.IsProduction() && len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters in production")
	}
	return nil
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envStringSlice(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func envInt(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return value, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration such as 24h: %w", key, err)
	}
	return value, nil
}
