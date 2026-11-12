package auth

import (
	"encoding/json"
	stderrors "errors"
	"net/http"

	"ai-buddy/core/internal/platform/errors"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *Service
	tokens  *TokenManager
}

func NewHandler(service *Service, tokens *TokenManager) *Handler {
	return &Handler{service: service, tokens: tokens}
}

func (h *Handler) Router() chi.Router {
	router := chi.NewRouter()

	router.Post("/auth/login", h.handleLogin)

	router.Group(func(r chi.Router) {
		r.Use(RequireAuth(h.tokens))
		r.Get("/me", h.handleMe)
	})

	return router
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
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

	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	principal, ok := PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	user, err := h.service.Me(r.Context(), principal.UserID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, user)
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
