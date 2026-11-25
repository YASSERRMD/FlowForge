// Command api runs the FlowForge HTTP and WebSocket API service.
//
//	api              # serve the API
//	api migrate      # apply database migrations (see migrate_cmd.go)
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/FlowForge/internal/config"
	"github.com/YASSERRMD/FlowForge/internal/engine"
	"github.com/YASSERRMD/FlowForge/internal/httpserver"
	"github.com/YASSERRMD/FlowForge/internal/workflowstore"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load(nil)
	if err != nil {
		log.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		os.Exit(runMigrate(cfg, os.Args[2:]))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var opts []httpserver.Option
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	opts = append(opts, httpserver.WithWorkflowStore(workflowstore.New(pool)))
	opts = append(opts, httpserver.WithEngine(engine.NewService(pool)))

	srv := httpserver.New(cfg, log, opts...)
	srv.RegisterReadinessCheck("postgres", func(ctx context.Context) error {
		return pool.Ping(ctx)
	})
	srv.RegisterReadinessCheck("redis", httpserver.DialCheck(cfg.RedisURL))
	if err := srv.Run(ctx); err != nil {
		log.Error("server exited with error", "error", err)
		os.Exit(1)
	}
	log.Info("server stopped cleanly")
}
