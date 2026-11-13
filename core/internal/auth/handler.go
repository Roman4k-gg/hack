package auth

import (
	"encoding/json"
	stderrors "errors"
	"log/slog"
	"net/http"

	"ai-buddy/core/internal/audit"
	"ai-buddy/core/internal/platform/errors"
	"ai-buddy/core/internal/platform/reqctx"
)

type Handler struct {
	service  *Service
	tokens   *TokenManager
	recorder *audit.Recorder
	logger   *slog.Logger
}

func NewHandler(service *Service, tokens *TokenManager, recorder *audit.Recorder, logger *slog.Logger) *Handler {
	return &Handler{service: service, tokens: tokens, recorder: recorder, logger: logger}
}

func (h *Handler) Login() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errors.Write(w, r.Context(), http.StatusBadRequest,
				errors.New(errors.CodeValidation, "Некорректный формат запроса").
					WithDetail("reason", err.Error()))
			return
		}

		session, err := h.service.Login(r.Context(), req.Email)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		h.record(r, session.User, "auth.login", "user_logged_in")

		writeJSON(w, http.StatusOK, session)
	}
}

func (h *Handler) Me() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := reqctx.PrincipalFrom(r.Context())
		if !ok {
			errors.Write(w, r.Context(), http.StatusUnauthorized,
				errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
			return
		}

		user, err := h.service.Me(r.Context(), principal.UserID, principal.Scopes)
		if err != nil {
			writeServiceError(w, r, err)
			return
		}

		writeJSON(w, http.StatusOK, user)
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) record(r *http.Request, user User, action, event string) {
	if h.recorder == nil {
		return
	}

	traceID := reqctx.TraceID(r.Context())
	payload := map[string]any{"email": user.Email, "role": user.Role}

	if err := h.recorder.Record(r.Context(), audit.Entry{
		ActorID:    user.ID,
		Action:     action,
		EntityType: "user",
		EntityID:   user.ID,
		Payload:    payload,
		TraceID:    traceID,
	}); err != nil && h.logger != nil {
		h.logger.Warn("cannot write audit log", "error", err.Error())
	}

	if err := h.recorder.Emit(r.Context(), audit.Event{
		UserID:     user.ID,
		Name:       event,
		EntityType: "user",
		EntityID:   user.ID,
		Payload:    payload,
		TraceID:    traceID,
	}); err != nil && h.logger != nil {
		h.logger.Warn("cannot write event", "error", err.Error())
	}
}

func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case stderrors.Is(err, ErrEmailRequired):
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Укажите email для входа"))
	case stderrors.Is(err, ErrInvalidEmail):
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Некорректный email"))
	case stderrors.Is(err, ErrUserNotFound):
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Пользователь не найден"))
	default:
		errors.Write(w, r.Context(), http.StatusInternalServerError,
			errors.New(errors.CodeInternal, "Внутренняя ошибка сервера"))
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
