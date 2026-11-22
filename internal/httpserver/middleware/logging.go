package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// sensitiveKeys are substrings that mark a query parameter as carrying
// credentials; matching values are redacted before logging.
var sensitiveKeys = []string{"token", "secret", "password", "api_key", "apikey", "key", "signature", "code"}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// RequestLogger emits one structured log line per request with the
// correlation ID, latency, matched route pattern and sanitized target.
// It never logs headers, cookies, bodies or raw query values that look
// like credentials.
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}

			next.ServeHTTP(rec, r)

			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}

			// The route pattern is only available after the handler ran.
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				routePattern = "unmatched"
			}

			level := slog.LevelInfo
			if status >= 500 {
				level = slog.LevelError
			} else if status >= 400 {
				level = slog.LevelWarn
			}

			log.Log(r.Context(), level, "http request",
				"request_id", FromContext(r.Context()),
				"method", r.Method,
				"route", routePattern,
				"path", r.URL.Path,
				"query", sanitizeQuery(r.URL.RawQuery),
				"status", status,
				"bytes", rec.bytes,
				"latency_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

// sanitizeQuery redacts the values of credential-looking query parameters.
func sanitizeQuery(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "&")
	for i, p := range parts {
		name, _, found := strings.Cut(p, "=")
		lower := strings.ToLower(name)
		for _, sensitive := range sensitiveKeys {
			if strings.Contains(lower, sensitive) {
				if found {
					parts[i] = name + "=REDACTED"
				}
				break
			}
		}
	}
	return strings.Join(parts, "&")
}
