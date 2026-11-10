package httpserver

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"ai-buddy/core/internal/platform/config"
	"ai-buddy/core/internal/platform/errors"
	"ai-buddy/core/internal/platform/reqctx"

	"github.com/go-chi/chi/v5"
)

func Recoverer(logger interface {
	Error(msg string, args ...any)
}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						"error", fmt.Sprintf("%v", rec),
						"method", r.Method,
						"route", routePattern(r),
						"trace_id", reqctx.TraceID(r.Context()),
					)
					errors.Write(w, r.Context(), http.StatusInternalServerError,
						errors.New(errors.CodeInternal, "Внутренняя ошибка сервера"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
			if id == "" {
				id = reqctx.NewTraceID()
			}
			w.Header().Set("X-Request-ID", id)
			next.ServeHTTP(w, r.WithContext(reqctx.WithTraceID(r.Context(), id)))
		})
	}
}

func AccessLog(logger interface {
	Info(msg string, args ...any)
}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			logger.Info("request",
				"trace_id", reqctx.TraceID(r.Context()),
				"user_id", reqctx.UserID(r.Context()),
				"method", r.Method,
				"route", routePattern(r),
				"status", rec.status,
				"latency_ms", time.Since(start).Milliseconds(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

func CORS(cfg config.Config) func(http.Handler) http.Handler {
	allowed := map[string]struct{}{}
	for _, o := range cfg.CORSAllowedOrigins {
		allowed[strings.ToLower(strings.TrimSpace(o))] = struct{}{}
	}
	anyOrigin := cfg.AllowsAnyOrigin()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				_, ok := allowed[strings.ToLower(origin)]
				if anyOrigin || ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "600")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if !r.written {
		r.status = status
		r.written = true
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.written = true
	}
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func routePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil || rctx.RoutePattern() == "" {
		return r.URL.Path
	}
	return rctx.RoutePattern()
}
