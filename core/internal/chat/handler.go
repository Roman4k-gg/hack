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
	history  HistoryStore
	recorder *audit.Recorder
	logger   *slog.Logger
}

func NewHandler(service *Service, history HistoryStore, recorder *audit.Recorder, logger *slog.Logger) *Handler {
	return &Handler{service: service, history: history, recorder: recorder, logger: logger}
}

func (h *Handler) Router() chi.Router {
	router := chi.NewRouter()

	router.With(rbac.RequireScopes(rbac.ScopeChat)).Post("/stream", h.handleStream)

	return router
}

func (h *Handler) HistoryRouter() chi.Router {
	router := chi.NewRouter()

	router.With(rbac.RequireScopes(rbac.ScopeChatHistory)).Get("/", h.handleConversations)

	router.With(rbac.RequireScopes(rbac.ScopeChatHistory)).Get("/{id}/messages", h.handleMessages)

	return router
}

type streamRequest struct {
	Message        string `json:"message"`
	ConversationID string `json:"conversation_id"`
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

	conversation, ok := h.conversation(w, r, principal, req)
	if !ok {
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
		"message_length":  len(req.Message),
		"question":        questionForAnalytics(req.Message),
		"conversation_id": conversation.ID,
	})

	h.saveMessage(r, conversation.ID, "user", req.Message, nil, nil)

	w.Header().Set("Content-Type", stream.ContentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flush(w)

	writeMeta(w, conversation.ID)

	collector := NewCollector()
	written := h.pipe(w, r, stream, collector)

	content, toolCalls, citations := collector.Result()

	if content != "" {
		h.saveMessage(r, conversation.ID, "assistant", content, toolCalls, citations)
	}

	h.recordEvent(r, principal, "chat_stream_finished", map[string]any{
		"bytes": written,
	})
}

func (h *Handler) conversation(w http.ResponseWriter, r *http.Request, principal reqctx.Principal, req streamRequest) (Conversation, bool) {
	if h.history == nil {
		return Conversation{}, true
	}

	ctx := r.Context()

	if strings.TrimSpace(req.ConversationID) == "" {
		conversation, err := h.history.CreateConversation(ctx, principal.UserID, conversationTitle(req.Message))
		if err != nil {
			h.logWarn("cannot create conversation", err)
			return Conversation{}, true
		}

		return conversation, true
	}

	conversation, err := h.history.Conversation(ctx, principal.UserID, req.ConversationID)
	if err != nil {
		if stderrors.Is(err, ErrConversationForbidden) {
			errors.Write(w, ctx, http.StatusForbidden,
				errors.New(errors.CodeForbidden, "Этот диалог принадлежит другому пользователю"))
			return Conversation{}, false
		}

		if stderrors.Is(err, ErrConversationNotFound) {
			errors.Write(w, ctx, http.StatusNotFound,
				errors.New(errors.CodeNotFound, "Диалог не найден"))
			return Conversation{}, false
		}

		h.logWarn("cannot load conversation", err)
		return Conversation{}, true
	}

	return conversation, true
}

func (h *Handler) saveMessage(
	r *http.Request,
	conversationID, role, content string,
	toolCalls []ToolCall,
	citations []Citation,
) {
	if h.history == nil || conversationID == "" {
		return
	}

	if err := h.history.AppendMessage(r.Context(), conversationID, role, content, toolCalls, citations); err != nil {
		h.logWarn("cannot save message", err)
	}
}

func (h *Handler) handleConversations(w http.ResponseWriter, r *http.Request) {
	principal, ok := reqctx.PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	if h.history == nil {
		writeJSON(w, http.StatusOK, []Conversation{})
		return
	}

	conversations, err := h.history.ListConversations(r.Context(), principal.UserID, 20)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, conversations)
}

func (h *Handler) handleMessages(w http.ResponseWriter, r *http.Request) {
	principal, ok := reqctx.PrincipalFrom(r.Context())
	if !ok {
		errors.Write(w, r.Context(), http.StatusUnauthorized,
			errors.New(errors.CodeUnauthorized, "Требуется авторизация"))
		return
	}

	if h.history == nil {
		writeJSON(w, http.StatusOK, []HistoryMessage{})
		return
	}

	conversationID := chi.URLParam(r, "id")

	if _, err := h.history.Conversation(r.Context(), principal.UserID, conversationID); err != nil {
		if stderrors.Is(err, ErrConversationForbidden) {
			errors.Write(w, r.Context(), http.StatusForbidden,
				errors.New(errors.CodeForbidden, "Этот диалог принадлежит другому пользователю"))
			return
		}

		if stderrors.Is(err, ErrConversationNotFound) {
			errors.Write(w, r.Context(), http.StatusNotFound,
				errors.New(errors.CodeNotFound, "Диалог не найден"))
			return
		}

		h.writeError(w, r, err)
		return
	}

	messages, err := h.history.Messages(r.Context(), conversationID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, messages)
}

func writeMeta(w http.ResponseWriter, conversationID string) {
	if conversationID == "" {
		return
	}

	payload, err := json.Marshal(map[string]any{
		"type":            "meta",
		"conversation_id": conversationID,
	})
	if err != nil {
		return
	}

	_, _ = w.Write([]byte("event: message\ndata: " + string(payload) + "\n\n"))
	flush(w)
}

func conversationTitle(message string) string {
	title := strings.TrimSpace(message)
	if len(title) > 60 {
		title = title[:60]
	}

	return title
}

func (h *Handler) pipe(w http.ResponseWriter, r *http.Request, stream *Stream, collector *Collector) int {
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

			if collector != nil {
				collector.Feed(chunk)
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
