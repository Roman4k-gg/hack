package kb

import (
	"encoding/json"
	stderrors "errors"
	"log/slog"
	"net/http"
	"strconv"

	"ai-buddy/core/internal/platform/errors"
	"ai-buddy/core/internal/platform/reqctx"

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

	router.Get("/search", h.handleSearch)
	router.Get("/documents/{id}", h.handleDocument)

	return router
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	principal, ok := reqctx.PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	if !accessFor(principal).Any() {
		errors.Write(w, r.Context(), http.StatusForbidden,
			errors.New(errors.CodeForbidden, "Недостаточно прав для чтения документов"))
		return
	}

	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			errors.Write(w, r.Context(), http.StatusBadRequest,
				errors.New(errors.CodeValidation, "Некорректный параметр limit"))
			return
		}
		limit = parsed
	}

	result, err := h.service.Search(r.Context(), principal, r.URL.Query().Get("q"), limit)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleDocument(w http.ResponseWriter, r *http.Request) {
	principal, ok := reqctx.PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	document, err := h.service.Document(r.Context(), principal, chi.URLParam(r, "id"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, document)
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case stderrors.Is(err, ErrEmptyQuery):
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Введите поисковый запрос"))
	case stderrors.Is(err, ErrDocumentNotFound):
		errors.Write(w, r.Context(), http.StatusNotFound,
			errors.New(errors.CodeNotFound, "Документ не найден"))
	case stderrors.Is(err, ErrDocumentForbidden):
		errors.Write(w, r.Context(), http.StatusForbidden,
			errors.New(errors.CodeForbidden, "Документ недоступен для вашей роли"))
	default:
		if h.logger != nil {
			h.logger.Error("kb request failed", "error", err.Error())
		}
		errors.Write(w, r.Context(), http.StatusInternalServerError,
			errors.New(errors.CodeInternal, "Не удалось выполнить поиск по документам"))
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
