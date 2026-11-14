package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-buddy/core/internal/analytics"
	"ai-buddy/core/internal/api"
	"ai-buddy/core/internal/audit"
	"ai-buddy/core/internal/auth"
	"ai-buddy/core/internal/chat"
	"ai-buddy/core/internal/flags"
	"ai-buddy/core/internal/integrations"
	"ai-buddy/core/internal/kb"
	"ai-buddy/core/internal/onboarding"
	"ai-buddy/core/internal/platform/config"
	"ai-buddy/core/internal/platform/db"
	"ai-buddy/core/internal/platform/httpserver"
	"ai-buddy/core/internal/platform/logging"
	"ai-buddy/core/internal/platform/ratelimit"
	"ai-buddy/core/internal/platform/redis"
	"ai-buddy/core/internal/requests"
	"ai-buddy/core/internal/team"
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

	if err := store.Migrate(ctx); err != nil {
		logger.Error("cannot apply migrations", "error", err.Error())
		os.Exit(1)
	}

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

	tokens, err := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL, cfg.JWTIssuer)
	if err != nil {
		logger.Error("cannot create token manager", "error", err.Error())
		os.Exit(1)
	}

	recorder := audit.New(store.Pool(), logger)

	authService := auth.NewService(auth.NewPostgresUserStore(store.Pool()), tokens)
	authHandler := auth.NewHandler(authService, tokens, recorder, logger)

	onboardingService := onboarding.NewService(onboarding.NewPostgresPlanStore(store.Pool()), recorder, logger)
	onboardingHandler := onboarding.NewHandler(onboardingService, logger)

	teamService := team.NewService(team.NewPostgresStore(store.Pool()))
	teamHandler := team.NewHandler(teamService, logger)

	kbService := kb.NewService(kb.NewPostgresStore(store.Pool()))
	kbHandler := kb.NewHandler(kbService, logger)

	absenceService := requests.NewAbsenceAdapter(integrations.NewMockAbsence(150 * time.Millisecond))
	requestsStore := requests.NewPostgresStore(store.Pool())
	requestsService := requests.NewService(
		requestsStore,
		requests.NewRedisTokenStore(cache.Client()),
		absenceService,
		requestsStore,
		recorder,
		logger,
	)
	requestsHandler := requests.NewHandler(requestsService, logger)

	flagsService := flags.NewService(flags.NewPostgresStore(store.Pool()))
	flagsHandler := flags.NewHandler(flagsService, logger)

	analyticsService := analytics.NewService(analytics.NewPostgresStore(store.Pool()))
	analyticsHandler := analytics.NewHandler(analyticsService, logger)

	limiter := ratelimit.New(ratelimit.NewRedisBackend(cache.Client()), cfg.RateLimitRPS)
	chatService := chat.NewService(chat.Config{
		AIURL:   cfg.AIURL,
		AIPath:  cfg.AIChatPath,
		Timeout: cfg.ChatTimeout,
		Limiter: limiter,
	}, chat.NewClient(), logger)
	chatHandler := chat.NewHandler(chatService, recorder, logger)

	srv.Mount("/api/v1", api.New(api.Deps{
		Auth:       authHandler,
		Onboarding: onboardingHandler,
		Chat:       chatHandler,
		Team:       teamHandler,
		KB:         kbHandler,
		Requests:   requestsHandler,
		Flags:      flagsHandler,
		Analytics:  analyticsHandler,
		Tokens:     tokens,
	}))

	if err := srv.Run(ctx); err != nil {
		logger.Error("core api stopped with error", "error", err.Error())
		os.Exit(1)
	}
	logger.Info("core api stopped")
}
