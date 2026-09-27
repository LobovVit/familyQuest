package config

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	RequireAccess bool
	AccessSecret  string
	SaaS          bool
	Migrate       bool
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
		RequireAccess: os.Getenv("FAMILYQUEST_REQUIRE_ACCESS") == "1",
		AccessSecret:  os.Getenv("ACCESS_SHARED_SECRET"),
		SaaS:          os.Getenv("FAMILYQUEST_SAAS") == "1",
		Migrate:       os.Getenv("FAMILYQUEST_SKIP_MIGRATIONS") != "1",
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://familyquest:familyquest@localhost:5433/familyquest?sslmode=disable"),
		HTTPAddr:      getEnv("HTTP_ADDR", ":8081"),
		CORSOrigin:    getEnv("CORS_ORIGIN", "*"),
		SeedFile:      getEnv("FAMILYQUEST_SEED_FILE", ""),
		SessionSecret: os.Getenv("SESSION_SECRET"),
		SessionTTL:    ttl,
	}
}

func (c Config) Validate() error {
	if c.RequireAccess && (!c.SaaS || c.Migrate || len(c.AccessSecret) < 32) {
		return errors.New("private API requires SaaS, disabled migrations and ACCESS_SHARED_SECRET of at least 32 characters")
	}
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
