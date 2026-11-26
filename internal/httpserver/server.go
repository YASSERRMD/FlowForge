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
	cfg       config.Config
	log       *slog.Logger
	router    chi.Router
	http      *http.Server
	checker   *Checker
	auth      Authenticator
	workflows WorkflowStore
	engine    EngineService
}

// Option customizes a Server at construction.
type Option func(*Server)

// WithWorkflowStore wires workflow persistence; without it the workflow
// routes answer 503.
func WithWorkflowStore(store WorkflowStore) Option {
	return func(s *Server) { s.workflows = store }
}

// WithEngine wires run execution; without it the run routes answer 503.
func WithEngine(svc EngineService) Option {
	return func(s *Server) { s.engine = svc }
}

// New builds the server with its route table and middleware stack.
func New(cfg config.Config, log *slog.Logger, opts ...Option) *Server {
	s := &Server{
		cfg:     cfg,
		log:     log,
		checker: NewChecker(2 * time.Second),
		auth:    NewAuthenticator(cfg),
	}
	for _, opt := range opts {
		opt(s)
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
			r.Get("/templates", s.handleListTemplates)
			r.Route("/workflows", func(r chi.Router) {
				r.With(s.requireWorkflowStore).Post("/", s.handleCreateWorkflow)
				r.With(s.requireWorkflowStore).Get("/", s.handleListWorkflows)
				r.With(s.requireWorkflowStore).Get("/{workflowID}", s.handleGetWorkflow)
				r.With(s.requireWorkflowStore).Patch("/{workflowID}", s.handleUpdateWorkflow)
				r.With(s.requireWorkflowStore).Delete("/{workflowID}", s.handleDeleteWorkflow)
				r.With(s.requireWorkflowStore).Put("/{workflowID}/draft", s.handleSaveDraft)
				r.With(s.requireWorkflowStore).Post("/{workflowID}/publish", s.handlePublish)
				r.With(s.requireWorkflowStore).Post("/{workflowID}/clone", s.handleClone)
				r.With(s.requireWorkflowStore).Get("/{workflowID}/versions", s.handleListVersions)
				r.With(s.requireWorkflowStore).Get("/{workflowID}/versions/{versionNumber}", s.handleGetVersion)
				r.With(s.requireWorkflowStore).Post("/{workflowID}/versions/{versionNumber}/copy-to-draft", s.handleCopyVersionToDraft)
				r.With(s.requireWorkflowStore, s.requireEngine).Post("/{workflowID}/runs", s.handleCreateRun)
			})
			r.With(s.requireWorkflowStore, s.requireEngine).Get("/runs/{runID}", s.handleGetRun)
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
