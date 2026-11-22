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

func TestLoadDevAuthMode(t *testing.T) {
	base := map[string]string{
		"DATABASE_URL":   "postgres://flowforge:flowforge@localhost:5432/flowforge",
		"REDIS_URL":      "redis://localhost:6379/0",
		"SESSION_SECRET": "dev-only-insecure-secret-change-me",
		"AUTH_DEV_MODE":  "true",
	}
	cfg, err := Load(envFunc(base))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.DevAuthEnabled {
		t.Fatal("DevAuthEnabled = false, want true")
	}

	base["APP_ENV"] = "production"
	base["SESSION_SECRET"] = strings.Repeat("x", 40)
	if _, err := Load(envFunc(base)); err == nil ||
		!strings.Contains(err.Error(), "AUTH_DEV_MODE") {
		t.Fatalf("want AUTH_DEV_MODE production rejection, got %v", err)
	}
}

func TestLoadHTTPTimeouts(t *testing.T) {
	cfg, err := Load(envFunc(map[string]string{
		"DATABASE_URL":          "postgres://flowforge:flowforge@localhost:5432/flowforge",
		"REDIS_URL":             "redis://localhost:6379/0",
		"SESSION_SECRET":        "dev-only-insecure-secret-change-me",
		"HTTP_READ_TIMEOUT_MS":  "5000",
		"HTTP_WRITE_TIMEOUT_MS": "9000",
		"HTTP_MAX_BODY_KB":      "256",
	}))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ReadTimeout.Milliseconds() != 5000 {
		t.Errorf("ReadTimeout = %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout.Milliseconds() != 9000 {
		t.Errorf("WriteTimeout = %v", cfg.WriteTimeout)
	}
	if cfg.MaxRequestBodyKB != 256 {
		t.Errorf("MaxRequestBodyKB = %d", cfg.MaxRequestBodyKB)
	}

	if _, err := Load(envFunc(map[string]string{
		"DATABASE_URL":         "postgres://flowforge:flowforge@localhost:5432/flowforge",
		"REDIS_URL":            "redis://localhost:6379/0",
		"SESSION_SECRET":       "dev-only-insecure-secret-change-me",
		"HTTP_READ_TIMEOUT_MS": "5",
	})); err == nil || !strings.Contains(err.Error(), "HTTP_READ_TIMEOUT_MS") {
		t.Fatalf("want timeout validation error, got %v", err)
	}
}
