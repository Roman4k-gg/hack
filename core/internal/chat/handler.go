package chat

import (
	"bufio"
	"encoding/json"
	stderrors "errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"ai-buddy/core/internal/audit"
	"ai-buddy/core/internal/platform/errors"
	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service  *Service
	recorder *audit.Recorder
	logger   *slog.Logger
}

func NewHandler(service *Service, recorder *audit.Recorder, logger *slog.Logger) *Handler {
	return &Handler{service: service, recorder: recorder, logger: logger}
}

func (h *Handler) Router() chi.Router {
	router := chi.NewRouter()

	router.With(rbac.RequireScopes(rbac.ScopeChat)).Post("/stream", h.handleStream)

	return router
}

type streamRequest struct {
	Message string `json:"message"`
}

func (h *Handler) handleStream(w http.ResponseWriter, r *http.Request) {
	principal, ok := reqctx.PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	var req streamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Некорректный формат запроса").
				WithDetail("reason", err.Error()))
		return
	}

	if req.Message == "" {
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Введите сообщение"))
		return
	}

	stream, err := h.service.Stream(r.Context(), principal, r.Header.Get("Authorization"), req.Message)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	defer stream.Cancel()
	defer stream.Body.Close()

	h.recordEvent(r, principal, "chat_stream_started", map[string]any{
		"message_length": len(req.Message),
		"question":       questionForAnalytics(req.Message),
	})

	w.Header().Set("Content-Type", stream.ContentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flush(w)

	written := h.pipe(w, r, stream)

	h.recordEvent(r, principal, "chat_stream_finished", map[string]any{
		"bytes": written,
	})
}

func (h *Handler) pipe(w http.ResponseWriter, r *http.Request, stream *Stream) int {
	reader := bufio.NewReader(stream.Body)
	written := 0

	for {
		chunk, err := reader.ReadBytes('\n')
		if len(chunk) > 0 {
			n, writeErr := w.Write(chunk)
			written += n
			flush(w)
			if writeErr != nil {
				break
			}
		}
		if err != nil {
			if !stderrors.Is(err, io.EOF) && r.Context().Err() == nil {
				h.logWarn("chat stream interrupted", err)
			}
			break
		}
		if r.Context().Err() != nil {
			break
		}
	}

	return written
}

const maxQuestionLength = 160

func questionForAnalytics(message string) string {
	question := strings.TrimSpace(message)
	if len(question) > maxQuestionLength {
		return question[:maxQuestionLength]
	}

	return question
}

func (h *Handler) recordEvent(r *http.Request, principal reqctx.Principal, name string, payload map[string]any) {
	if h.recorder == nil {
		return
	}
	err := h.recorder.Emit(r.Context(), audit.Event{
		UserID:     principal.UserID,
		Name:       name,
		EntityType: "chat",
		EntityID:   principal.UserID,
		Payload:    payload,
		TraceID:    reqctx.TraceID(r.Context()),
	})
	if err != nil {
		h.logWarn("cannot write event", err)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case stderrors.Is(err, ErrEmptyMessage):
		errors.Write(w, r.Context(), http.StatusBadRequest,
			errors.New(errors.CodeValidation, "Введите сообщение"))
	case stderrors.Is(err, ErrRateLimited):
		errors.Write(w, r.Context(), http.StatusTooManyRequests,
			errors.New(errors.CodeUnavailable, "Слишком много запросов, попробуйте позже"))
	case stderrors.Is(err, ErrUpstreamTimeout):
		errors.Write(w, r.Context(), http.StatusGatewayTimeout,
			errors.New(errors.CodeTimeout, "Ответ ассистента не успел прийти вовремя"))
	case stderrors.Is(err, ErrUpstreamStatus):
		errors.Write(w, r.Context(), http.StatusBadGateway,
			errors.New(errors.CodeUpstreamError, "Ассистент сейчас недоступен"))
	case stderrors.Is(err, ErrUpstream):
		errors.Write(w, r.Context(), http.StatusServiceUnavailable,
			errors.New(errors.CodeUnavailable, "Ассистент сейчас недоступен"))
	default:
		h.logWarn("chat request failed", err)
		errors.Write(w, r.Context(), http.StatusServiceUnavailable,
			errors.New(errors.CodeUnavailable, "Ассистент сейчас недоступен"))
	}
}

func (h *Handler) logWarn(message string, err error) {
	if h.logger == nil {
		return
	}
	h.logger.Warn(message, "error", err.Error())
}

func flush(w http.ResponseWriter) {
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
