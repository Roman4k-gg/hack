package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"ai-buddy/core/internal/platform/config"
	"ai-buddy/core/internal/platform/db"
	"ai-buddy/core/internal/platform/logging"
	"ai-buddy/core/internal/seed"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed: invalid configuration: %v\n", err)
		os.Exit(1)
	}

	logger := logging.New(cfg.LogLevel, "seed")
	slog.SetDefault(logger)

	store, err := db.New(ctx, cfg.PostgresDSN, cfg.DBMaxConns)
	if err != nil {
		logger.Error("cannot create postgres pool", "error", err.Error())
		os.Exit(1)
	}
	defer store.Close()

	if err := store.Migrate(ctx); err != nil {
		logger.Error("cannot apply migrations", "error", err.Error())
		os.Exit(1)
	}

	if err := seed.Run(ctx, store.Pool(), seed.Options{Demo: cfg.SeedDemo}); err != nil {
		logger.Error("cannot seed demo data", "error", err.Error())
		os.Exit(1)
	}

	if cfg.SeedDemo {
		logger.Info("seed completed", "demo", true)
	} else {
		logger.Info("seed completed", "demo", false, "reason", "SEED_DEMO is disabled")
	}
}
