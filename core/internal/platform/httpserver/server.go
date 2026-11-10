package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"ai-buddy/core/internal/platform/config"
	"ai-buddy/core/internal/platform/db"
	"ai-buddy/core/internal/platform/redis"

	"github.com/go-chi/chi/v5"
)

var Version = "dev"

type Server struct {
	cfg    config.Config
	store  *db.Store
	cache  *redis.Client
	logger *slog.Logger
	http   *http.Server
}

func New(cfg config.Config, store *db.Store, cache *redis.Client, logger *slog.Logger) *Server {
	s := &Server{
		cfg:    cfg,
		store:  store,
		cache:  cache,
		logger: logger,
	}

	router := chi.NewRouter()

	router.Use(Recoverer(logger))
	router.Use(RequestID())
	router.Use(AccessLog(logger))
	router.Use(CORS(cfg))

	router.Get("/healthz", s.handleHealthz)
	router.Get("/readyz", s.handleReadyz)

	s.http = &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	return s
}

func (s *Server) Handler() http.Handler {
	return s.http.Handler
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		s.logger.Info("core api started", "addr", s.cfg.HTTPAddr, "version", Version)
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err, ok := <-errCh:
		if ok && err != nil {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()
		s.logger.Info("core api shutting down")
		if err := s.http.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	}
}
