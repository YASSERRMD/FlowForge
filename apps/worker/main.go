// Command worker runs the FlowForge workflow execution worker: it claims
// durable tasks from Postgres, executes graph nodes and serves liveness and
// readiness probes.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/FlowForge/internal/buildinfo"
	"github.com/YASSERRMD/FlowForge/internal/config"
	"github.com/YASSERRMD/FlowForge/internal/engine"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load(nil)
	if err != nil {
		log.Error("load config", "error", err)
		os.Exit(1)
	}
	log.Info("starting flowforge-worker",
		"version", buildinfo.Version, "commit", buildinfo.Commit, "workerId", cfg.WorkerID)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Error("ping database", "error", err)
		os.Exit(1)
	}

	health := startHealthServer(cfg.WorkerHealthAddr, pool, log)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = health.Shutdown(shutdownCtx)
	}()

	worker := engine.NewWorker(engine.WorkerConfig{
		ID:              cfg.WorkerID,
		Concurrency:     cfg.WorkerConcurrency,
		PollInterval:    cfg.PollInterval,
		ShutdownTimeout: cfg.ShutdownTimeout,
	}, engine.NewQueue(pool), engine.NewDispatcher(pool, log), log)

	if err := worker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("worker stopped", "error", err)
		os.Exit(1)
	}
	log.Info("worker stopped cleanly")
}

// startHealthServer serves /healthz (process up) and /readyz (database
// reachable) for the worker process.
func startHealthServer(addr string, pool *pgxpool.Pool, log *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		w.Header().Set("Content-Type", "application/json")
		if err := pool.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unavailable", "postgres": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("health server", "error", err)
		}
	}()
	return srv
}
