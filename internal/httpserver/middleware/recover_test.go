package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecovererReturnsSafe500(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	h := RequestID(Recoverer(log)(http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) {
			panic("database connection string leaked here")
		},
	)))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}

	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json body: %v", err)
	}
	if body.Error.Code != "internal_error" {
		t.Errorf("code = %q", body.Error.Code)
	}
	if body.Error.RequestID == "" {
		t.Error("request_id missing from error body")
	}
	if strings.Contains(rec.Body.String(), "database connection string") {
		t.Fatal("panic detail leaked to client")
	}

	// Server log must carry the diagnostic with correlation ID.
	if !strings.Contains(buf.String(), "database connection string") {
		t.Fatal("panic detail missing from server log")
	}
	if !strings.Contains(buf.String(), body.Error.RequestID) {
		t.Fatal("request id missing from server log")
	}
	if !strings.Contains(buf.String(), "stack") {
		t.Fatal("stack trace missing from server log")
	}
}

func TestRecovererPassesThroughNormalRequests(t *testing.T) {
	log := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	h := Recoverer(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want 418", rec.Code)
	}
}
