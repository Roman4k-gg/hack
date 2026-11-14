package flags

import (
	"context"
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

	router.With(rbac.RequireScopes(rbac.ScopeAdminFlags)).
		Get("/", h.handleList)

	router.With(rbac.RequireScopes(rbac.ScopeAdminFlags)).
		Patch("/{key}", h.handleUpdate)

	return router
}

type updateRequest struct {
	Enabled *bool `json:"enabled"`
	Rollout *int  `json:"rollout"`
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	flags, err := h.service.List(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, flags)
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	principal, ok := reqctx.PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	key := chi.URLParam(r, "key")

	var input updateRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Некорректный формат запроса").
				WithDetail("reason", err.Error()))
		return
	}

	enabled, rollout, err := h.current(r.Context(), key)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	if input.Rollout != nil {
		rollout = *input.Rollout
	}

	flag, err := h.service.Set(r.Context(), key, enabled, rollout, principal.UserID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, flag)
}

func (h *Handler) current(ctx context.Context, key string) (bool, int, error) {
	flags, err := h.service.List(ctx)
	if err != nil {
		return false, 0, err
	}

	for _, flag := range flags {
		if flag.Key == key {
			return flag.Enabled, flag.Rollout, nil
		}
	}

	return true, 100, nil
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case stderrors.Is(err, ErrKeyRequired):
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Укажите ключ флага"))
	case stderrors.Is(err, ErrRolloutRange):
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Rollout должен быть от 0 до 100"))
	default:
		if h.logger != nil {
			h.logger.Error("flags request failed", "error", err.Error())
		}
		errors.Write(w, r.Context(), http.StatusInternalServerError,
			errors.New(errors.CodeInternal, "Не удалось обновить флаги"))
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
