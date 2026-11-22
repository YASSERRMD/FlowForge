package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLivenessAlwaysOK(t *testing.T) {
	srv := New(testConfig(), testLogger())
	srv.RegisterReadinessCheck("failing", func(context.Context) error {
		return errors.New("down")
	})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200 even with failing deps", rec.Code)
	}
}

func TestReadinessOKWhenAllChecksPass(t *testing.T) {
	srv := New(testConfig(), testLogger())
	srv.RegisterReadinessCheck("a", func(context.Context) error { return nil })
	srv.RegisterReadinessCheck("b", func(context.Context) error { return nil })

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("readyz status = %d", rec.Code)
	}
	var body readinessBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body.Status != "ok" || body.Checks["a"] != "ok" || body.Checks["b"] != "ok" {
		t.Errorf("body = %+v", body)
	}
}

func TestReadiness503WhenCheckFails(t *testing.T) {
	srv := New(testConfig(), testLogger())
	srv.RegisterReadinessCheck("ok-dep", func(context.Context) error { return nil })
	srv.RegisterReadinessCheck("bad-dep", func(context.Context) error {
		return errors.New("connection refused")
	})

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status = %d, want 503", rec.Code)
	}
	var body readinessBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body.Status != "degraded" {
		t.Errorf("status = %q", body.Status)
	}
	if body.Checks["ok-dep"] != "ok" {
		t.Errorf("ok-dep = %q", body.Checks["ok-dep"])
	}
	if body.Checks["bad-dep"] == "ok" {
		t.Errorf("bad-dep unexpectedly ok: %+v", body.Checks)
	}
}

func TestReadinessCheckTimeoutIsBounded(t *testing.T) {
	srv := New(testConfig(), testLogger())
	srv.checker.timeout = 100 * time.Millisecond
	srv.RegisterReadinessCheck("slow", func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
			return nil
		}
	})

	start := time.Now()
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status = %d, want 503", rec.Code)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("readiness took %v, want bounded by check timeout", elapsed)
	}
}

func TestDialCheckReachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	check := DialCheck("postgres://" + ln.Addr().String() + "/db")
	if err := check(context.Background()); err != nil {
		t.Fatalf("DialCheck against live listener: %v", err)
	}
}

func TestDialCheckUnreachable(t *testing.T) {
	// Grab a port then close it so nothing listens there.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	check := DialCheck("redis://" + addr + "/0")
	if err := check(context.Background()); err == nil {
		t.Fatal("DialCheck succeeded against closed port")
	}
}

func TestDialCheckAddsDefaultPorts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	// localhost with default scheme ports — we only assert it does not
	// return a URL parse error (connection outcome depends on the host).
	check := DialCheck("postgres://localhost")
	err := check(ctx)
	if err != nil && err.Error() == "missing port in address" {
		t.Fatalf("default port not applied: %v", err)
	}
}
