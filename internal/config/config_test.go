package config

import (
	"strings"
	"testing"
)

func envFunc(kv map[string]string) func(string) string {
	return func(key string) string { return kv[key] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(envFunc(map[string]string{
		"DATABASE_URL":   "postgres://flowforge:flowforge@localhost:5432/flowforge",
		"REDIS_URL":      "redis://localhost:6379/0",
		"SESSION_SECRET": "dev-only-insecure-secret-change-me",
	}))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Env != EnvDevelopment {
		t.Errorf("Env = %q, want %q", cfg.Env, EnvDevelopment)
	}
	if cfg.APIAddr != "127.0.0.1:8080" {
		t.Errorf("APIAddr = %q", cfg.APIAddr)
	}
	if cfg.WorkerConcurrency != 8 {
		t.Errorf("WorkerConcurrency = %d, want 8", cfg.WorkerConcurrency)
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "http://localhost:3000" {
		t.Errorf("AllowedOrigins = %v", cfg.AllowedOrigins)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	_, err := Load(envFunc(map[string]string{"REDIS_URL": "redis://localhost:6379"}))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL is required") {
		t.Fatalf("want DATABASE_URL error, got %v", err)
	}
}

func TestLoadRejectsWeakProductionSecret(t *testing.T) {
	_, err := Load(envFunc(map[string]string{
		"APP_ENV":        "production",
		"DATABASE_URL":   "postgres://x",
		"REDIS_URL":      "redis://x",
		"SESSION_SECRET": "dev-only-insecure-secret-change-me",
	}))
	if err == nil || !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Fatalf("want SESSION_SECRET error, got %v", err)
	}
}

func TestLoadRejectsPlainHttpOriginInProduction(t *testing.T) {
	_, err := Load(envFunc(map[string]string{
		"APP_ENV":             "production",
		"DATABASE_URL":        "postgres://x",
		"REDIS_URL":           "redis://x",
		"SESSION_SECRET":      strings.Repeat("a", 40),
		"API_ALLOWED_ORIGINS": "https://app.example.com,http://insecure.example.com",
	}))
	if err == nil || !strings.Contains(err.Error(), "https in production") {
		t.Fatalf("want https origin error, got %v", err)
	}
}

func TestLoadRejectsBadConcurrency(t *testing.T) {
	_, err := Load(envFunc(map[string]string{
		"DATABASE_URL":       "postgres://x",
		"REDIS_URL":          "redis://x",
		"WORKER_CONCURRENCY": "0",
	}))
	if err == nil || !strings.Contains(err.Error(), "WORKER_CONCURRENCY") {
		t.Fatalf("want WORKER_CONCURRENCY error, got %v", err)
	}
}

func TestLoadProductionValid(t *testing.T) {
	cfg, err := Load(envFunc(map[string]string{
		"APP_ENV":             "production",
		"DATABASE_URL":        "postgres://x",
		"REDIS_URL":           "redis://x",
		"SESSION_SECRET":      strings.Repeat("s", 48),
		"API_ALLOWED_ORIGINS": "https://app.example.com",
	}))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Env != EnvProduction {
		t.Errorf("Env = %q", cfg.Env)
	}
}
