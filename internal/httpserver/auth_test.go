package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YASSERRMD/FlowForge/internal/config"
)

func TestDevModeAuthenticatesWithHeader(t *testing.T) {
	cfg := testConfig()
	cfg.DevAuthEnabled = true
	srv := New(cfg, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set(DevUserHeader, "ada")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var identity Identity
	if err := json.Unmarshal(rec.Body.Bytes(), &identity); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if identity.Subject != "dev:ada" {
		t.Errorf("subject = %q", identity.Subject)
	}
}

func TestDevModeRejectsMissingHeader(t *testing.T) {
	cfg := testConfig()
	cfg.DevAuthEnabled = true
	srv := New(cfg, testLogger())

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestProductionFailsClosedWithoutAuth(t *testing.T) {
	cfg := testConfig()
	cfg.Env = config.EnvProduction
	cfg.DevAuthEnabled = false
	srv := New(cfg, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	// Even a dev header must be ignored: no authenticator is configured.
	req.Header.Set(DevUserHeader, "mallory")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (fail closed)", rec.Code)
	}
}

func TestProductionRejectsDevAuthConfig(t *testing.T) {
	cfg := testConfig()
	cfg.Env = config.EnvProduction
	cfg.DevAuthEnabled = true // invalid combo; config.Load rejects it
	if auth := NewAuthenticator(cfg); auth != nil {
		t.Fatalf("NewAuthenticator returned %T for production dev-mode combo", auth)
	}
}

func TestPublicPingNeedsNoAuth(t *testing.T) {
	cfg := testConfig()
	cfg.Env = config.EnvProduction
	srv := New(cfg, testLogger())

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("ping status = %d, want 200", rec.Code)
	}
}
