package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env             string
	Host            string
	Port            int
	SessionSecret   string
	SessionLifetime time.Duration
	DBPath          string
	LogLevel        string
	AdminEmail      string
	AdminPassword   string
	DeepSeekAPIKey  string
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
		DeepSeekAPIKey:  getenv("DEEPSEEK_API_KEY", ""),
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
