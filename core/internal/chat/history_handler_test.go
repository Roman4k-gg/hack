package chat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/stretchr/testify/require"
)

type fakeHistory struct {
	mu        sync.Mutex
	createErr error
	loadErr   error
	listErr   error
	readErr   error
	appendErr error
	saved     []string
}

func (f *fakeHistory) CreateConversation(_ context.Context, _, title string) (Conversation, error) {
	if f.createErr != nil {
		return Conversation{}, f.createErr
	}
	return Conversation{ID: "conv-new", Title: title}, nil
}

func (f *fakeHistory) Conversation(_ context.Context, _, conversationID string) (Conversation, error) {
	if f.loadErr != nil {
		return Conversation{}, f.loadErr
	}
	return Conversation{ID: conversationID}, nil
}

func (f *fakeHistory) ListConversations(_ context.Context, _ string, _ int) ([]Conversation, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return []Conversation{{ID: "conv-1", Title: "Про VPN"}}, nil
}

func (f *fakeHistory) AppendMessage(_ context.Context, _, role, _ string, _ []ToolCall, _ []Citation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved = append(f.saved, role)
	return f.appendErr
}

func (f *fakeHistory) Messages(_ context.Context, _ string) ([]HistoryMessage, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	return []HistoryMessage{{Role: "user", Content: "Привет"}}, nil
}

func historyHandler(history HistoryStore, upstreamURL string) *Handler {
	service := NewService(Config{AIURL: upstreamURL, Timeout: 5 * time.Second}, NewClient(), nil)
	return NewHandler(service, history, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func principalRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	return req.WithContext(reqctx.WithPrincipal(req.Context(), reqctx.Principal{
		UserID: "user-1",
		Role:   rbac.RoleIntern,
		Scopes: []string{rbac.ScopeChat, rbac.ScopeChatHistory},
	}))
}

func historyRequest(handler *Handler, path string, withPrincipal bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if withPrincipal {
		req = principalRequest(http.MethodGet, path, "")
	}
	rec := httptest.NewRecorder()
	handler.HistoryRouter().ServeHTTP(rec, req)
	return rec
}

func TestHistoryEndpoints(t *testing.T) {
	t.Parallel()

	handler := historyHandler(&fakeHistory{}, "http://127.0.0.1:1")

	rec := historyRequest(handler, "/", true)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Про VPN")

	rec = historyRequest(handler, "/conv-1/messages", true)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Привет")

	require.NotEqual(t, http.StatusOK, historyRequest(handler, "/", false).Code)
	require.NotEqual(t, http.StatusOK, historyRequest(handler, "/conv-1/messages", false).Code)
}

func TestHistoryEndpointsWithoutStore(t *testing.T) {
	t.Parallel()

	handler := historyHandler(nil, "http://127.0.0.1:1")

	require.Equal(t, "[]\n", historyRequest(handler, "/", true).Body.String())
	require.Equal(t, "[]\n", historyRequest(handler, "/conv-1/messages", true).Body.String())
}

func TestHistoryEndpointErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		history *fakeHistory
		path    string
		status  int
	}{
		{&fakeHistory{listErr: errors.New("db")}, "/", http.StatusServiceUnavailable},
		{&fakeHistory{loadErr: ErrConversationForbidden}, "/conv-1/messages", http.StatusForbidden},
		{&fakeHistory{loadErr: ErrConversationNotFound}, "/conv-1/messages", http.StatusNotFound},
		{&fakeHistory{loadErr: errors.New("db")}, "/conv-1/messages", http.StatusServiceUnavailable},
		{&fakeHistory{readErr: errors.New("db")}, "/conv-1/messages", http.StatusServiceUnavailable},
	}

	for _, item := range cases {
		rec := historyRequest(historyHandler(item.history, "http://127.0.0.1:1"), item.path, true)
		require.Equal(t, item.status, rec.Code, item.path)
	}
}

func sseUpstream(t *testing.T, bodies chan<- map[string]any) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies <- body
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: message\ndata: {\"type\":\"delta\",\"text\":\"Ответ\"}\n\n"))
	}))
	t.Cleanup(server.Close)

	return server
}

func streamWith(handler *Handler, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.Router().ServeHTTP(rec, principalRequest(http.MethodPost, "/stream", body))
	return rec
}

func TestStreamSavesConversation(t *testing.T) {
	t.Parallel()

	bodies := make(chan map[string]any, 2)
	upstream := sseUpstream(t, bodies)
	history := &fakeHistory{}
	handler := historyHandler(history, upstream.URL)

	rec := streamWith(handler, `{"message":"Где VPN?"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"conversation_id":"conv-new"`)
	require.Equal(t, "conv-new", (<-bodies)["conversation_id"])

	rec = streamWith(handler, `{"message":"А ещё?","conversation_id":"conv-7"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "conv-7", (<-bodies)["conversation_id"])
	require.Equal(t, []string{"user", "assistant", "user", "assistant"}, history.saved)
}

func TestStreamConversationErrors(t *testing.T) {
	t.Parallel()

	bodies := make(chan map[string]any, 4)
	upstream := sseUpstream(t, bodies)

	forbidden := streamWith(historyHandler(&fakeHistory{loadErr: ErrConversationForbidden}, upstream.URL),
		`{"message":"x","conversation_id":"c"}`)
	require.Equal(t, http.StatusForbidden, forbidden.Code)

	missing := streamWith(historyHandler(&fakeHistory{loadErr: ErrConversationNotFound}, upstream.URL),
		`{"message":"x","conversation_id":"c"}`)
	require.Equal(t, http.StatusNotFound, missing.Code)

	broken := &fakeHistory{loadErr: errors.New("db"), createErr: errors.New("db"), appendErr: errors.New("db")}
	for _, body := range []string{`{"message":"x","conversation_id":"c"}`, `{"message":"x"}`} {
		rec := streamWith(historyHandler(broken, upstream.URL), body)
		require.Equal(t, http.StatusOK, rec.Code)
		require.NotContains(t, rec.Body.String(), "conversation_id")
		require.NotContains(t, <-bodies, "conversation_id")
	}
}
