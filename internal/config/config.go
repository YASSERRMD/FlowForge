// Package config loads and validates typed environment configuration for the
// FlowForge Go services. Invalid or missing settings fail fast at startup
// instead of surfacing as runtime surprises.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Env is the deployment environment.
type Env string

const (
	// EnvDevelopment enables local-only conveniences such as the dev login.
	EnvDevelopment Env = "development"
	// EnvProduction disables every anonymous or development-only path.
	EnvProduction Env = "production"
)

// Config is the validated runtime configuration shared by api and worker.
type Config struct {
	Env               Env
	APIAddr           string
	AllowedOrigins    []string
	DatabaseURL       string
	RedisURL          string
	WorkerID          string
	WorkerConcurrency int
	PollInterval      time.Duration
	// WorkerHealthAddr is the worker's liveness/readiness listener.
	WorkerHealthAddr string
	SessionSecret    string
	// DevAuthEnabled allows the explicit development-only auth bypass. It is
	// rejected in production regardless of intent.
	DevAuthEnabled bool
	// HTTP server timeouts.
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	IdleTimeout      time.Duration
	ShutdownTimeout  time.Duration
	MaxRequestBodyKB int
}

const (
	minSessionSecretLen   = 32
	defaultPollInterval   = 250 * time.Millisecond
	defaultConcurrency    = 8
	defaultReadTimeout    = 30 * time.Second
	defaultWriteTimeout   = 60 * time.Second
	defaultIdleTimeout    = 120 * time.Second
	defaultShutdownTimout = 10 * time.Second
	defaultMaxBodyKB      = 1024
)

// Load reads configuration from the process environment and validates it.
func Load(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	var errs []string

	cfg := Config{
		APIAddr:          valueOr(getenv("API_ADDR"), "127.0.0.1:8080"),
		DatabaseURL:      getenv("DATABASE_URL"),
		RedisURL:         getenv("REDIS_URL"),
		WorkerID:         valueOr(getenv("WORKER_ID"), "worker-local-1"),
		WorkerHealthAddr: valueOr(getenv("WORKER_HEALTH_ADDR"), "127.0.0.1:8081"),
		SessionSecret:    getenv("SESSION_SECRET"),
		PollInterval:     defaultPollInterval,
		ReadTimeout:      defaultReadTimeout,
		WriteTimeout:     defaultWriteTimeout,
		IdleTimeout:      defaultIdleTimeout,
		ShutdownTimeout:  defaultShutdownTimout,
		MaxRequestBodyKB: defaultMaxBodyKB,
	}

	cfg.DevAuthEnabled = valueOr(getenv("AUTH_DEV_MODE"), "false") == "true"
	if cfg.DevAuthEnabled && Env(valueOr(getenv("APP_ENV"), string(EnvDevelopment))) == EnvProduction {
		errs = append(errs, "AUTH_DEV_MODE must not be enabled in production")
	}

	if v := getenv("HTTP_READ_TIMEOUT_MS"); v != "" {
		d, err := durationMillis(v, 1000)
		if err != nil {
			errs = append(errs, "HTTP_READ_TIMEOUT_MS must be an integer >= 1000")
		} else {
			cfg.ReadTimeout = d
		}
	}
	if v := getenv("HTTP_WRITE_TIMEOUT_MS"); v != "" {
		d, err := durationMillis(v, 1000)
		if err != nil {
			errs = append(errs, "HTTP_WRITE_TIMEOUT_MS must be an integer >= 1000")
		} else {
			cfg.WriteTimeout = d
		}
	}
	if v := getenv("HTTP_MAX_BODY_KB"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 102400 {
			errs = append(errs, "HTTP_MAX_BODY_KB must be an integer between 1 and 102400")
		} else {
			cfg.MaxRequestBodyKB = n
		}
	}

	switch env := Env(valueOr(getenv("APP_ENV"), string(EnvDevelopment))); env {
	case EnvDevelopment, EnvProduction:
		cfg.Env = env
	default:
		errs = append(errs, fmt.Sprintf("APP_ENV must be %q or %q, got %q", EnvDevelopment, EnvProduction, env))
	}

	origins := strings.Split(valueOr(getenv("API_ALLOWED_ORIGINS"), "http://localhost:3000"), ",")
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if _, err := url.ParseRequestURI(o); err != nil {
			errs = append(errs, fmt.Sprintf("API_ALLOWED_ORIGINS entry %q is not a valid origin URL", o))
			continue
		}
		cfg.AllowedOrigins = append(cfg.AllowedOrigins, o)
	}
	if len(cfg.AllowedOrigins) == 0 {
		errs = append(errs, "API_ALLOWED_ORIGINS must list at least one origin")
	}

	if cfg.DatabaseURL == "" {
		errs = append(errs, "DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		errs = append(errs, "REDIS_URL is required")
	}

	if v := getenv("WORKER_CONCURRENCY"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 256 {
			errs = append(errs, "WORKER_CONCURRENCY must be an integer between 1 and 256")
		} else {
			cfg.WorkerConcurrency = n
		}
	} else {
		cfg.WorkerConcurrency = defaultConcurrency
	}

	if v := getenv("WORKER_POLL_INTERVAL_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 10 {
			errs = append(errs, "WORKER_POLL_INTERVAL_MS must be an integer >= 10")
		} else {
			cfg.PollInterval = time.Duration(n) * time.Millisecond
		}
	}

	if cfg.Env == EnvProduction {
		if len(cfg.SessionSecret) < minSessionSecretLen ||
			strings.Contains(cfg.SessionSecret, "dev-only") ||
			strings.Contains(cfg.SessionSecret, "change-me") {
			errs = append(errs, "SESSION_SECRET must be a random secret of at least 32 bytes in production")
		}
		for _, o := range cfg.AllowedOrigins {
			if strings.HasPrefix(o, "http://") && !strings.HasPrefix(o, "http://localhost") && !strings.HasPrefix(o, "http://127.") {
				errs = append(errs, fmt.Sprintf("API_ALLOWED_ORIGINS entry %q must use https in production", o))
			}
		}
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return cfg, nil
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func durationMillis(v string, min int) (time.Duration, error) {
	n, err := strconv.Atoi(v)
	if err != nil || n < min {
		return 0, fmt.Errorf("invalid millisecond value %q", v)
	}
	return time.Duration(n) * time.Millisecond, nil
}
