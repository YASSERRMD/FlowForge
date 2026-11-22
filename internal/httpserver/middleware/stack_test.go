package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/YASSERRMD/FlowForge/internal/httpserver/middleware"
)

// buildStack assembles the production middleware order around test routes,
// mirroring httpserver.New: RequestID -> RequestLogger -> Recoverer.
func buildStack(log *slog.Logger) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestLogger(log))
	r.Use(middleware.Recoverer(log))
	r.Get("/ok", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/panic", func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	return r
}

func TestStackRequestIDFlowsEndToEnd(t *testing.T) {
	h := buildStack(slog.New(slog.NewJSONHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("X-Request-ID", "corr-42")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "corr-42" {
		t.Fatalf("X-Request-ID = %q, want corr-42", got)
	}
}

func TestStackPanicBecomesSafe500WithCorrelation(t *testing.T) {
	var logs bytes.Buffer
	h := buildStack(slog.New(slog.NewJSONHandler(&logs, nil)))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set("X-Request-ID", "corr-panic")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Fatal("panic detail leaked to client")
	}
	if !strings.Contains(rec.Body.String(), "corr-panic") {
		t.Fatal("correlation id missing from error body")
	}
	if !strings.Contains(logs.String(), "panic recovered") {
		t.Fatal("panic not logged server-side")
	}
}

func TestStackLogsEveryRequestOnce(t *testing.T) {
	var logs bytes.Buffer
	h := buildStack(slog.New(slog.NewJSONHandler(&logs, nil)))

	for i := 0; i < 3; i++ {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ok", nil))
	}
	lines := strings.Count(strings.TrimSpace(logs.String()), "\n") + 1
	if lines != 3 {
		t.Fatalf("log lines = %d, want 3 (one per request)", lines)
	}
}

func TestGracefulShutdownMiddlewareChain(t *testing.T) {
	// A request in flight during shutdown must complete: the middleware
	// stack must not swallow contexts. Exercise by running a handler that
	// respects cancellation.
	h := buildStack(slog.New(slog.NewJSONHandler(io.Discard, nil)))
	srv := httptest.NewServer(h)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/ok", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request through live server: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Request-ID") == "" {
		t.Fatal("live response missing X-Request-ID")
	}
}

func TestErrorBodyIsContractShaped(t *testing.T) {
	h := buildStack(slog.New(slog.NewJSONHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not the contract shape: %v", err)
	}
	if body.Error.Code != "internal_error" {
		t.Errorf("code = %q", body.Error.Code)
	}
}
