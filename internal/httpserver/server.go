// Package httpserver wires the FlowForge HTTP API: versioned routing,
// middleware, and graceful shutdown.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/YASSERRMD/FlowForge/internal/config"
	"github.com/YASSERRMD/FlowForge/internal/httpserver/middleware"
)

// Server is the FlowForge API HTTP server.
type Server struct {
	cfg     config.Config
	log     *slog.Logger
	router  chi.Router
	http    *http.Server
	checker *Checker
	auth    Authenticator
}

// New builds the server with its route table and middleware stack.
func New(cfg config.Config, log *slog.Logger) *Server {
	s := &Server{
		cfg:     cfg,
		log:     log,
		checker: NewChecker(2 * time.Second),
		auth:    NewAuthenticator(cfg),
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestLogger(log))
	r.Use(middleware.Recoverer(log))
	r.Get("/healthz", s.handleLiveness)
	r.Get("/readyz", s.handleReadiness)
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/ping", s.handlePing)
		r.Group(func(r chi.Router) {
			r.Use(s.RequireAuth)
			r.Get("/me", s.handleMe)
		})
	})
	s.router = r

	s.http = &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return s
}

// Handler exposes the root handler (used in tests).
func (s *Server) Handler() http.Handler { return s.router }

// RegisterReadinessCheck adds a dependency probe to /readyz.
func (s *Server) RegisterReadinessCheck(name string, fn CheckFunc) {
	s.checker.Register(name, fn)
}

// Run starts serving and blocks until ctx is cancelled, then shuts down
// gracefully within a bounded drain period.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("api listening", "addr", s.http.Addr)
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return <-errCh
}

func (s *Server) handlePing(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
