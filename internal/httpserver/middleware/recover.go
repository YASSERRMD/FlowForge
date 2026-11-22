package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recoverer turns handler panics into safe 500 responses. The panic value
// and stack trace go only to the server log (with the correlation ID); the
// client receives a generic body that leaks no internals.
func Recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.ErrorContext(r.Context(), "panic recovered",
						"request_id", FromContext(r.Context()),
						"panic", fmt.Sprintf("%v", rec),
						"stack", string(debug.Stack()),
					)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, `{"error":{"code":"internal_error","message":"internal server error","request_id":%q}}`,
						FromContext(r.Context()))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
