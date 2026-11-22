package httpserver

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// CheckFunc verifies one dependency. It must honor ctx deadlines.
type CheckFunc func(ctx context.Context) error

// Checker runs named dependency checks with an overall per-check timeout.
type Checker struct {
	checks  map[string]CheckFunc
	timeout time.Duration
}

// NewChecker builds a Checker. timeout bounds each individual check.
func NewChecker(timeout time.Duration) *Checker {
	return &Checker{checks: map[string]CheckFunc{}, timeout: timeout}
}

// Register adds a dependency check under a stable name.
func (c *Checker) Register(name string, fn CheckFunc) {
	c.checks[name] = fn
}

type readinessBody struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

func (c *Checker) run(ctx context.Context) readinessBody {
	results := make(map[string]string, len(c.checks))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, fn := range c.checks {
		wg.Add(1)
		go func(name string, fn CheckFunc) {
			defer wg.Done()
			checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
			defer cancel()
			status := "ok"
			if err := fn(checkCtx); err != nil {
				status = "error: " + err.Error()
			}
			mu.Lock()
			results[name] = status
			mu.Unlock()
		}(name, fn)
	}
	wg.Wait()

	body := readinessBody{Status: "ok", Checks: results}
	for _, status := range results {
		if status != "ok" {
			body.Status = "degraded"
			break
		}
	}
	return body
}

// LivenessHandler reports only that the process is up; it must never
// depend on external systems.
func (s *Server) handleLiveness(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// ReadinessHandler runs every registered dependency check and returns 503
// when any of them fails.
func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	body := s.checker.run(r.Context())
	status := http.StatusOK
	if body.Status != "ok" {
		status = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// DialCheck builds a dependency check that dials the host:port of the given
// URL. It verifies network reachability only; driver-level pings replace it
// once the storage layer lands.
func DialCheck(rawURL string) CheckFunc {
	return func(ctx context.Context) error {
		u, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		host := u.Host
		if _, _, err := net.SplitHostPort(host); err != nil {
			switch u.Scheme {
			case "postgres", "postgresql":
				host += ":5432"
			case "redis", "rediss":
				host += ":6379"
			default:
				return err
			}
		}
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", host)
		if err != nil {
			return err
		}
		return conn.Close()
	}
}
