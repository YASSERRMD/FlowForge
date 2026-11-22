package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRequestIDGeneratedAndEchoed(t *testing.T) {
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if FromContext(r.Context()) == "" {
			t.Error("request id missing from context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	id := rec.Header().Get(RequestIDHeader)
	if id == "" {
		t.Fatal("response missing X-Request-ID")
	}
	if len(id) != 32 {
		t.Fatalf("generated id length = %d, want 32 hex chars", len(id))
	}
}

func TestRequestIDHonorsIncomingHeader(t *testing.T) {
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := FromContext(r.Context()); got != "incoming-123" {
			t.Errorf("context id = %q", got)
		}
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "incoming-123")
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get(RequestIDHeader); got != "incoming-123" {
		t.Fatalf("echoed id = %q", got)
	}
}

func TestRequestLoggerSanitizesCredentials(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	r := chi.NewRouter()
	r.Use(RequestID)
	r.Use(RequestLogger(log))
	r.Get("/api/v1/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/ping?access_token=supersecret&name=flow", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	line := buf.String()
	var entry map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &entry); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, line)
	}
	if strings.Contains(line, "supersecret") {
		t.Fatalf("credential leaked into log line: %s", line)
	}
	if !strings.Contains(line, "REDACTED") {
		t.Fatalf("expected redaction marker in: %s", line)
	}
	if entry["route"] != "/api/v1/ping" {
		t.Errorf("route = %v", entry["route"])
	}
	if entry["status"] != float64(200) {
		t.Errorf("status = %v", entry["status"])
	}
	if _, ok := entry["latency_ms"]; !ok {
		t.Error("latency_ms missing")
	}
	if entry["request_id"] == "" {
		t.Error("request_id missing")
	}
}

func TestFromContextWithoutMiddleware(t *testing.T) {
	if got := FromContext(context.Background()); got != "" {
		t.Fatalf("FromContext = %q, want empty", got)
	}
}
