package analytics

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"ai-buddy/core/internal/platform/errors"
	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) Router() chi.Router {
	router := chi.NewRouter()

	router.With(rbac.RequireScopes(rbac.ScopeAnalyticsTeam)).
		Get("/top-questions", h.handleTopQuestions)

	router.With(rbac.RequireScopes(rbac.ScopeAnalyticsTeam)).
		Get("/content-gaps", h.handleContentGaps)

	return router
}

func (h *Handler) AdminRouter() chi.Router {
	router := chi.NewRouter()

	router.With(rbac.RequireScopes(rbac.ScopeAnalyticsTeam)).
		Get("/overview", h.handleOverview)

	return router
}

func (h *Handler) handleTopQuestions(w http.ResponseWriter, r *http.Request) {
	if _, ok := reqctx.PrincipalFrom(r.Context()); !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	questions, err := h.service.TopQuestions(r.Context(), limitOf(r))
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, questions)
}

func (h *Handler) handleContentGaps(w http.ResponseWriter, r *http.Request) {
	if _, ok := reqctx.PrincipalFrom(r.Context()); !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	gaps, err := h.service.ContentGaps(r.Context(), limitOf(r))
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, gaps)
}

func (h *Handler) handleOverview(w http.ResponseWriter, r *http.Request) {
	if _, ok := reqctx.PrincipalFrom(r.Context()); !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	overview, err := h.service.Overview(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, overview)
}

func limitOf(r *http.Request) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return 0
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}

	return value
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	if h.logger != nil {
		h.logger.Error("analytics request failed", "error", err.Error())
	}

	errors.Write(w, r.Context(), http.StatusInternalServerError,
		errors.New(errors.CodeInternal, "Не удалось собрать аналитику"))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
