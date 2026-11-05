package config

import (
	"strings"
	"testing"
)

func TestProductionConfigurationHasNoImplicitDatabaseOrSeed(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if Load().DatabaseURL != "" {
		t.Fatal("embedded database credentials")
	}
	c := Config{SessionSecret: strings.Repeat("s", 32), SessionTTL: 1}
	if c.Validate() == nil {
		t.Fatal("missing database accepted")
	}
	c.DatabaseURL, c.SaaS, c.SeedFile = "postgres://configured", true, "seed/development.json"
	if c.Validate() == nil {
		t.Fatal("SaaS accepted bootstrap seed")
	}
	c.SeedFile = ""
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}
