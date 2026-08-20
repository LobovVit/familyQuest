package config

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	DatabaseURL   string
	HTTPAddr      string
	CORSOrigin    string
	SeedFile      string
	SessionSecret string
	SessionTTL    time.Duration
}

func Load() Config {
	ttl := 12 * time.Hour
	if raw := os.Getenv("SESSION_TTL"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil {
			ttl = parsed
		} else {
			ttl = 0
		}
	}
	return Config{
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://familyquest:familyquest@localhost:5433/familyquest?sslmode=disable"),
		HTTPAddr:      getEnv("HTTP_ADDR", ":8081"),
		CORSOrigin:    getEnv("CORS_ORIGIN", "*"),
		SeedFile:      getEnv("FAMILYQUEST_SEED_FILE", ""),
		SessionSecret: os.Getenv("SESSION_SECRET"),
		SessionTTL:    ttl,
	}
}

func (c Config) Validate() error {
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if len(c.SessionSecret) < 32 {
		return errors.New("SESSION_SECRET must be at least 32 characters")
	}
	if c.SessionTTL <= 0 {
		return errors.New("SESSION_TTL must be a positive duration")
	}
	return nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
