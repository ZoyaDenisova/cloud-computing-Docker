package config

import (
	"strings"
	"testing"
)

func TestLoadRequiresCredentials(t *testing.T) {
	t.Setenv("DB_USER", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_NAME", "")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error when credentials are missing")
	}
}

func TestLoadUsesDefaultsAndBuildsDSN(t *testing.T) {
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_PORT", "")
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "todo")
	t.Setenv("APP_PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if cfg.DBHost != "db" {
		t.Fatalf("expected default DB_HOST=db, got %q", cfg.DBHost)
	}
	if cfg.DBPort != "5432" {
		t.Fatalf("expected default DB_PORT=5432, got %q", cfg.DBPort)
	}
	if cfg.AppPort != "8080" {
		t.Fatalf("expected default APP_PORT=8080, got %q", cfg.AppPort)
	}

	dsn := cfg.DSN()
	if !strings.Contains(dsn, "host=db") || !strings.Contains(dsn, "dbname=todo") {
		t.Fatalf("unexpected DSN: %q", dsn)
	}
}

func TestGetEnv(t *testing.T) {
	t.Setenv("SOME_ENV", "value")
	if got := getEnv("SOME_ENV", "fallback"); got != "value" {
		t.Fatalf("expected value, got %q", got)
	}

	t.Setenv("EMPTY_ENV", "")
	if got := getEnv("EMPTY_ENV", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}
