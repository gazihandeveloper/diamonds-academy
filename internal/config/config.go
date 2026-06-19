package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env              string
	Host             string
	Port             int
	SessionSecret    string
	SessionLifetime  time.Duration
	DBPath           string
	LogLevel         string
	AdminEmail       string
	AdminPassword    string
	DeepSeekAPIKey       string
	GoogleClientID       string
	GoogleClientSecret   string
	GoogleRedirectURL    string
	AppleTeamID          string
	AppleServiceID       string
	AppleKeyID           string
	ApplePrivateKey      string
	AppleRedirectURL     string
	InstagramClientID    string
	InstagramClientSecret string
	InstagramRedirectURL string
	USDTRYRate           float64
}

func Load() (*Config, error) {
	_ = godotenv.Load() // .env opsiyonel

	port, err := strconv.Atoi(getenv("APP_PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid APP_PORT: %w", err)
	}

	lifeH, _ := strconv.Atoi(getenv("SESSION_LIFETIME_HOURS", "24"))
	if lifeH <= 0 {
		lifeH = 24
	}

	cfg := &Config{
		Env:             getenv("APP_ENV", "development"),
		Host:            getenv("APP_HOST", "127.0.0.1"),
		Port:            port,
		SessionSecret:   getenv("SESSION_SECRET", "dev-secret-change-me-in-prod-please!"),
		SessionLifetime: time.Duration(lifeH) * time.Hour,
		DBPath:          getenv("DB_PATH", "./data/diamonds.db"),
		LogLevel:        getenv("LOG_LEVEL", "info"),
		AdminEmail:      getenv("ADMIN_EMAIL", "admin@diamondsacademy.com"),
		AdminPassword:   getenv("ADMIN_PASSWORD", "diamondsadmin"),
		DeepSeekAPIKey:     getenv("DEEPSEEK_API_KEY", ""),
		GoogleClientID:      getenv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:  getenv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:   getenv("GOOGLE_REDIRECT_URL", ""),
		AppleTeamID:         getenv("APPLE_TEAM_ID", ""),
		AppleServiceID:      getenv("APPLE_SERVICE_ID", ""),
		AppleKeyID:          getenv("APPLE_KEY_ID", ""),
		ApplePrivateKey:     getenv("APPLE_PRIVATE_KEY", ""),
		AppleRedirectURL:    getenv("APPLE_REDIRECT_URL", ""),
		InstagramClientID:   getenv("INSTAGRAM_CLIENT_ID", ""),
		InstagramClientSecret: getenv("INSTAGRAM_CLIENT_SECRET", ""),
		InstagramRedirectURL: getenv("INSTAGRAM_REDIRECT_URL", ""),
		USDTRYRate:      32.0,
	}
	if v := getenv("USD_TRY_RATE", ""); v != "" {
		if r, err := strconv.ParseFloat(v, 64); err == nil && r > 0 {
			cfg.USDTRYRate = r
		}
	}

	if len(cfg.SessionSecret) < 16 {
		return nil, fmt.Errorf("SESSION_SECRET must be at least 16 chars")
	}
	return cfg, nil
}

func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c *Config) IsDev() bool { return c.Env == "development" }

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
