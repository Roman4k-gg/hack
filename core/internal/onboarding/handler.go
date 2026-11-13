package onboarding

import (
	"encoding/json"
	stderrors "errors"
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

	router.With(rbac.RequireScopes(rbac.ScopePlanRead)).Get("/plan", h.handlePlan)
	router.With(rbac.RequireScopes(rbac.ScopePlanWrite)).Patch("/items/{id}", h.handleUpdateItem)

	return router
}

func (h *Handler) handlePlan(w http.ResponseWriter, r *http.Request) {
	principal, ok := reqctx.PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	plan, err := h.service.Plan(r.Context(), principal.UserID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, plan)
}

func (h *Handler) handleUpdateItem(w http.ResponseWriter, r *http.Request) {
	principal, ok := reqctx.PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	var input UpdateItemInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Некорректный формат запроса").
				WithDetail("reason", err.Error()))
		return
	}

	item, err := h.service.UpdateItem(r.Context(), principal, chi.URLParam(r, "id"), input)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case stderrors.Is(err, ErrPlanNotFound):
		errors.Write(w, r.Context(), http.StatusNotFound,
			errors.New(errors.CodeNotFound, "План адаптации не найден"))
	case stderrors.Is(err, ErrItemNotFound):
		errors.Write(w, r.Context(), http.StatusNotFound,
			errors.New(errors.CodeNotFound, "Пункт плана не найден"))
	case stderrors.Is(err, ErrForbidden):
		errors.Write(w, r.Context(), http.StatusForbidden,
			errors.New(errors.CodeForbidden, "Недостаточно прав для этого действия"))
	case stderrors.Is(err, ErrInvalidStatus):
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Некорректный статус пункта"))
	case stderrors.Is(err, ErrMissingReason):
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Укажите причину блокировки"))
	default:
		if h.logger != nil {
			h.logger.Error("onboarding request failed", "error", err.Error())
		}
		errors.Write(w, r.Context(), http.StatusInternalServerError,
			errors.New(errors.CodeInternal, "Внутренняя ошибка сервера"))
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
