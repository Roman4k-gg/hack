package requests

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

	router.With(rbac.RequireScopes(rbac.ScopeAbsenceSelf)).
		Post("/absence/preview", h.handlePreview)

	router.With(rbac.RequireScopes(rbac.ScopeAbsenceSelfWrite)).
		Post("/absence", h.handleCreate)

	router.With(rbac.RequireScopes(rbac.ScopeRequestSelf)).
		Get("/", h.handleList)

	router.With(rbac.RequireScopes(rbac.ScopeRequestSelf)).
		Get("/{id}", h.handleGet)

	return router
}

type previewRequest struct {
	DateFrom    string `json:"date_from"`
	DateTo      string `json:"date_to"`
	AbsenceType string `json:"absence_type"`
	Comment     string `json:"comment"`
}

type createRequest struct {
	PreviewID         string `json:"preview_id"`
	ConfirmationToken string `json:"confirmation_token"`
	IdempotencyKey    string `json:"idempotency_key"`
}

func (h *Handler) handlePreview(w http.ResponseWriter, r *http.Request) {
	principal, ok := reqctx.PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	var input previewRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Некорректный формат запроса").
				WithDetail("reason", err.Error()))
		return
	}

	preview, err := h.service.PreviewAbsence(r.Context(), principal, AbsenceInput{
		DateFrom:    input.DateFrom,
		DateTo:      input.DateTo,
		AbsenceType: input.AbsenceType,
		Comment:     input.Comment,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, preview)
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := reqctx.PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	var input createRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Некорректный формат запроса").
				WithDetail("reason", err.Error()))
		return
	}

	detail, err := h.service.CreateAbsence(r.Context(), principal,
		input.PreviewID, input.ConfirmationToken, input.IdempotencyKey)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	principal, ok := reqctx.PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	list, err := h.service.List(r.Context(), principal)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, list)
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	principal, ok := reqctx.PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	detail, err := h.service.Get(r.Context(), principal, chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case stderrors.Is(err, ErrInvalidDates):
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Проверьте даты отпуска"))
	case stderrors.Is(err, ErrInvalidType):
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Неизвестный тип отпуска"))
	case stderrors.Is(err, ErrCommentTooLong):
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Комментарий слишком длинный"))
	case stderrors.Is(err, ErrMissingToken):
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Нужно подтверждение действия"))
	case stderrors.Is(err, ErrMissingKey):
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Передайте idempotency_key"))
	case stderrors.Is(err, ErrInvalidToken):
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Токен подтверждения недействителен или истёк"))
	case stderrors.Is(err, ErrForbidden):
		errors.Write(w, r.Context(), http.StatusForbidden,
			errors.New(errors.CodeForbidden, "Недостаточно прав для этого действия"))
	case stderrors.Is(err, ErrRequestNotFound):
		errors.Write(w, r.Context(), http.StatusNotFound,
			errors.New(errors.CodeNotFound, "Заявка не найдена"))
	default:
		if h.logger != nil {
			h.logger.Error("requests request failed", "error", err.Error())
		}
		errors.Write(w, r.Context(), http.StatusBadGateway,
			errors.New(errors.CodeUpstreamError, "Сервис отсутствий сейчас недоступен"))
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
