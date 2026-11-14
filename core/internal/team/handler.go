package team

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"log/slog"
	"net/http"

	"ai-buddy/core/internal/onboarding"
	"ai-buddy/core/internal/platform/errors"
	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/go-chi/chi/v5"
)

type PlanProvider interface {
	PlanForIntern(ctx context.Context, principal reqctx.Principal, internID string) (onboarding.Plan, error)
}

type Handler struct {
	service *Service
	plans   PlanProvider
	logger  *slog.Logger
}

func NewHandler(service *Service, plans PlanProvider, logger *slog.Logger) *Handler {
	return &Handler{service: service, plans: plans, logger: logger}
}

func (h *Handler) Router() chi.Router {
	router := chi.NewRouter()

	router.With(rbac.RequireScopes(rbac.ScopeTeamRead)).Get("/interns", h.handleInterns)
	router.With(rbac.RequireScopes(rbac.ScopeAnalyticsTeam)).Get("/overview", h.handleOverview)
	router.With(rbac.RequireScopes(rbac.ScopeTeamRead)).Get("/interns/{id}/plan", h.handleInternPlan)

	return router
}

func (h *Handler) handleInternPlan(w http.ResponseWriter, r *http.Request) {
	principal, ok := reqctx.PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	if h.plans == nil {
		errors.Write(w, r.Context(), http.StatusServiceUnavailable,
			errors.New(errors.CodeUnavailable, "Планы адаптации временно недоступны"))
		return
	}

	plan, err := h.plans.PlanForIntern(r.Context(), principal, chi.URLParam(r, "id"))
	if err != nil {
		h.writePlanError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, plan)
}

func (h *Handler) writePlanError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case stderrors.Is(err, onboarding.ErrPlanNotFound):
		errors.Write(w, r.Context(), http.StatusNotFound,
			errors.New(errors.CodeNotFound, "Стажёр не найден"))
	case stderrors.Is(err, onboarding.ErrForbidden):
		errors.Write(w, r.Context(), http.StatusForbidden,
			errors.New(errors.CodeForbidden, "Этот стажёр не в вашей команде"))
	default:
		if h.logger != nil {
			h.logger.Error("team plan request failed", "error", err.Error())
		}
		errors.Write(w, r.Context(), http.StatusInternalServerError,
			errors.New(errors.CodeInternal, "Не удалось загрузить план адаптации"))
	}
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
