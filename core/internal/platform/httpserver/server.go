package httpserver

import (
	"context"
	stderrors "errors"
	"log/slog"
	"net/http"
	"time"

	"ai-buddy/core/internal/platform/config"
	"ai-buddy/core/internal/platform/db"
	"ai-buddy/core/internal/platform/errors"
	"ai-buddy/core/internal/platform/redis"

	"github.com/go-chi/chi/v5"
)

var Version = "dev"

type Server struct {
	cfg    config.Config
	store  *db.Store
	cache  *redis.Client
	logger *slog.Logger
	router chi.Router
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

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		errors.Write(w, r.Context(), http.StatusNotFound,
			errors.New(errors.CodeNotFound, "Ресурс не найден"))
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		errors.Write(w, r.Context(), http.StatusMethodNotAllowed,
			errors.New(errors.CodeValidation, "Метод не поддерживается"))
	})

	s.router = router

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

func (s *Server) Mount(pattern string, handler chi.Router) {
	s.router.Mount(pattern, handler)
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		s.logger.Info("core api started", "addr", s.cfg.HTTPAddr, "version", Version)
		if err := s.http.ListenAndServe(); err != nil && !stderrors.Is(err, http.ErrServerClosed) {
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
