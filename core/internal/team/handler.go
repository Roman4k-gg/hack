package team

import (
	"encoding/json"
	"log/slog"
	"net/http"

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

	router.With(rbac.RequireScopes(rbac.ScopeTeamRead)).Get("/interns", h.handleInterns)
	router.With(rbac.RequireScopes(rbac.ScopeAnalyticsTeam)).Get("/overview", h.handleOverview)

	return router
}

func (h *Handler) handleInterns(w http.ResponseWriter, r *http.Request) {
	principal, ok := reqctx.PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	report, err := h.service.Report(r.Context(), principal)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func (h *Handler) handleOverview(w http.ResponseWriter, r *http.Request) {
	principal, ok := reqctx.PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	report, err := h.service.Report(r.Context(), principal)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, report.Overview)
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	if h.logger != nil {
		h.logger.Error("team request failed", "error", err.Error())
	}

	errors.Write(w, r.Context(), http.StatusInternalServerError,
		errors.New(errors.CodeInternal, "Не удалось собрать данные по команде"))
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
