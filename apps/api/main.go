// Command api runs the FlowForge HTTP and WebSocket API service.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/YASSERRMD/FlowForge/internal/config"
	"github.com/YASSERRMD/FlowForge/internal/httpserver"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load(nil)
	if err != nil {
		log.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := httpserver.New(cfg, log)
	if err := srv.Run(ctx); err != nil {
		log.Error("server exited with error", "error", err)
		os.Exit(1)
	}
	log.Info("server stopped cleanly")
}
