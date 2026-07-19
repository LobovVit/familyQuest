package config

import "os"

type Config struct {
	DatabaseURL string
	HTTPAddr    string
	CORSOrigin  string
}

func Load() Config {
	return Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://familyquest:familyquest@localhost:5433/familyquest?sslmode=disable"),
		HTTPAddr:    getEnv("HTTP_ADDR", ":8081"),
		CORSOrigin:  getEnv("CORS_ORIGIN", "*"),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
