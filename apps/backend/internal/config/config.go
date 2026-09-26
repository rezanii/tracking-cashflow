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
	DBSSLMode  string

	JWTSecret     string
	JWTExpiration time.Duration

	CORSAllowedOrigins []string

	Telegram TelegramConfig
}

// TelegramMode selects how updates reach the bot.
type TelegramMode string

const (
	// TelegramModeOff disables the integration entirely.
	TelegramModeOff TelegramMode = "off"
	// TelegramModePolling pulls updates with getUpdates, which needs no public URL.
	TelegramModePolling TelegramMode = "polling"
	// TelegramModeWebhook expects Telegram to POST to this service over HTTPS.
	TelegramModeWebhook TelegramMode = "webhook"
)

type TelegramConfig struct {
	BotToken string
	Mode     TelegramMode
	// WebhookSecret is compared against X-Telegram-Bot-Api-Secret-Token. Telegram sends
	// whatever was passed to setWebhook, so an attacker who guesses the URL still cannot
	// inject updates.
	WebhookSecret string
	// WebhookURL is only used by the setWebhook helper.
	WebhookURL string
	// APIBaseURL is overridable so tests can point the client at a local stub.
	APIBaseURL string
	// PairingCodeTTL bounds how long a pairing code stays usable.
	PairingCodeTTL time.Duration
}

// Enabled reports whether the bot should do anything at all.
func (t TelegramConfig) Enabled() bool {
	return t.BotToken != "" && t.Mode != TelegramModeOff
}

func (c Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

// PostgresDSN builds a connection URL. Credentials are escaped so a password containing
// reserved characters cannot break or alter the connection string.
//
// DatabaseName is a parameter so the migrator can reach the maintenance database without
// building a second config by hand.
func (c Config) PostgresDSN() string {
	return c.postgresDSN(c.DBName)
}

// MaintenanceDSN points at the "postgres" database, which always exists. CREATE DATABASE
// cannot run from inside the database being created.
func (c Config) MaintenanceDSN() string {
	return c.postgresDSN("postgres")
}

func (c Config) postgresDSN(database string) string {
	query := url.Values{}
	// Hosted Postgres (Neon, Supabase, Render) refuses plaintext, and local Docker has no
	// certificate, so the mode is configurable rather than assumed.
	query.Set("sslmode", c.DBSSLMode)

	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.DBUser, c.DBPassword),
		Host:     fmt.Sprintf("%s:%d", c.DBHost, c.DBPort),
		Path:     "/" + database,
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
		DBSSLMode:          envString("DB_SSLMODE", "disable"),
		JWTSecret:          envString("JWT_SECRET", ""),
		CORSAllowedOrigins: envStringSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		Telegram: TelegramConfig{
			BotToken:      envString("TELEGRAM_BOT_TOKEN", ""),
			Mode:          TelegramMode(strings.ToLower(envString("TELEGRAM_MODE", string(TelegramModeOff)))),
			WebhookSecret: envString("TELEGRAM_WEBHOOK_SECRET", ""),
			WebhookURL:    envString("TELEGRAM_WEBHOOK_URL", ""),
			APIBaseURL:    strings.TrimSuffix(envString("TELEGRAM_API_BASE_URL", "https://api.telegram.org"), "/"),
		},
	}

	var err error
	if cfg.AppPort, err = envInt("APP_PORT", 8080); err != nil {
		return Config{}, err
	}
	if cfg.DBPort, err = envInt("DB_PORT", 5432); err != nil {
		return Config{}, err
	}
	if cfg.JWTExpiration, err = envDuration("JWT_EXPIRATION", 24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.Telegram.PairingCodeTTL, err = envDuration("TELEGRAM_PAIRING_CODE_TTL", 15*time.Minute); err != nil {
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
	switch c.DBSSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		return fmt.Errorf("DB_SSLMODE must be one of disable, allow, prefer, require, verify-ca, verify-full, got %q", c.DBSSLMode)
	}
	// Sending credentials in the clear to a hosted database is not something to fall back to
	// silently, so production has to ask for TLS.
	if c.IsProduction() && (c.DBSSLMode == "disable" || c.DBSSLMode == "allow") {
		return fmt.Errorf("DB_SSLMODE must require TLS in production, got %q", c.DBSSLMode)
	}
	return c.Telegram.validate()
}

func (t TelegramConfig) validate() error {
	switch t.Mode {
	case TelegramModeOff, TelegramModePolling, TelegramModeWebhook:
	default:
		return fmt.Errorf("TELEGRAM_MODE must be off, polling or webhook, got %q", t.Mode)
	}
	if t.Mode != TelegramModeOff && t.BotToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is required when TELEGRAM_MODE is %s", t.Mode)
	}
	// Without a secret any host that learns the URL could post forged updates, so the
	// webhook is refused rather than exposed.
	if t.Mode == TelegramModeWebhook && t.WebhookSecret == "" {
		return fmt.Errorf("TELEGRAM_WEBHOOK_SECRET is required when TELEGRAM_MODE is webhook")
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
