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
	"ai-buddy/core/internal/platform/httpserver"
	"ai-buddy/core/internal/platform/logging"
	"ai-buddy/core/internal/platform/redis"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "core-api: invalid configuration: %v\n", err)
		os.Exit(1)
	}

	logger := logging.New(cfg.LogLevel, "core")
	slog.SetDefault(logger)

	store, err := db.New(ctx, cfg.PostgresDSN, cfg.DBMaxConns)
	if err != nil {
		logger.Error("cannot create postgres pool", "error", err.Error())
		os.Exit(1)
	}
	defer store.Close()

	cache, err := redis.New(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		logger.Error("cannot create redis client", "error", err.Error())
		os.Exit(1)
	}
	defer func() {
		if err := cache.Close(); err != nil {
			logger.Error("cannot close redis client", "error", err.Error())
		}
	}()

	srv := httpserver.New(cfg, store, cache, logger)

	if err := srv.Run(ctx); err != nil {
		logger.Error("core api stopped with error", "error", err.Error())
		os.Exit(1)
	}
	logger.Info("core api stopped")
}
