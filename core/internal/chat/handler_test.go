package chat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-buddy/core/internal/platform/ratelimit"
	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/stretchr/testify/require"
)

type fakeBackend struct {
	count int64
	err   error
}

func (b *fakeBackend) Incr(_ context.Context, _ string) (int64, error) {
	if b.err != nil {
		return 0, b.err
	}
	b.count++
	return b.count, nil
}

func (b *fakeBackend) Expire(_ context.Context, _ string, _ time.Duration) error {
	return b.err
}

func newTestHandler(t *testing.T, upstreamURL string, limit int) *Handler {
	t.Helper()

	limiter := ratelimit.New(&fakeBackend{}, limit)
	service := NewService(Config{
		AIURL:   upstreamURL,
		AIPath:  "/api/v1/chat/stream",
		Timeout: 5 * time.Second,
		Limiter: limiter,
	}, NewClient(), nil)

	return NewHandler(service, nil, nil)
}

func chatRequest(t *testing.T, handler *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/stream", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer token-from-client")
	req = req.WithContext(reqctx.WithPrincipal(req.Context(), reqctx.Principal{
		UserID: "user-1",
		Email:  "dasha@demo.local",
		Role:   rbac.RoleIntern,
		Scopes: []string{rbac.ScopeChat},
	}))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	return rec
}

func TestStreamProxiesUpstreamEvents(t *testing.T) {
	t.Parallel()

	var gotMethod, gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("event: token\ndata: {\"text\":\"Привет\"}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("event: done\ndata: {}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	handler := newTestHandler(t, upstream.URL, 0)

	rec := chatRequest(t, handler, `{"message":"привет"}`)

	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, "Bearer token-from-client", gotAuth)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	require.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	require.Contains(t, rec.Body.String(), "event: token")
	require.Contains(t, rec.Body.String(), "event: done")
}

func TestStreamRejectsEmptyMessage(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, "http://127.0.0.1:1", 0)

	rec := chatRequest(t, handler, `{"message":"   "}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestStreamRequiresScope(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, "http://127.0.0.1:1", 0)

	req := httptest.NewRequest(http.MethodPost, "/stream", strings.NewReader(`{"message":"привет"}`))
	req = req.WithContext(reqctx.WithPrincipal(req.Context(), reqctx.Principal{
		UserID: "user-1",
		Role:   rbac.RoleIntern,
		Scopes: []string{rbac.ScopePlanRead},
	}))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestStreamReturnsBadGatewayOnUpstreamError(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer upstream.Close()

	handler := newTestHandler(t, upstream.URL, 0)

	rec := chatRequest(t, handler, `{"message":"привет"}`)

	require.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestStreamReturnsGatewayTimeoutOnSlowUpstream(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	service := NewService(Config{
		AIURL:   upstream.URL,
		AIPath:  "/api/v1/chat/stream",
		Timeout: 20 * time.Millisecond,
	}, NewClient(), nil)
	handler := NewHandler(service, nil, nil)

	rec := chatRequest(t, handler, `{"message":"привет"}`)

	require.Equal(t, http.StatusGatewayTimeout, rec.Code)
}

func TestStreamIsRateLimited(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: done\ndata: {}\n\n"))
	}))
	defer upstream.Close()

	backend := &fakeBackend{}
	service := NewService(Config{
		AIURL:   upstream.URL,
		AIPath:  "/api/v1/chat/stream",
		Timeout: 5 * time.Second,
		Limiter: ratelimit.New(backend, 1),
	}, NewClient(), nil)
	handler := NewHandler(service, nil, nil)

	rec := chatRequest(t, handler, `{"message":"первый"}`)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = chatRequest(t, handler, `{"message":"второй"}`)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestStreamUrl(t *testing.T) {
	t.Parallel()

	service := NewService(Config{AIURL: "http://ai:8000/", AIPath: "v1/chat/stream"}, nil, nil)
	require.Equal(t, "http://ai:8000/v1/chat/stream", service.url())

	service = NewService(Config{AIURL: "http://ai:8000"}, nil, nil)
	require.Equal(t, "http://ai:8000/api/v1/chat/stream", service.url())
}

func TestServiceStreamRejectsEmptyMessage(t *testing.T) {
	t.Parallel()

	service := NewService(Config{}, NewClient(), nil)

	_, err := service.Stream(context.Background(), reqctx.Principal{UserID: "user-1"}, "", "  ")
	require.True(t, errors.Is(err, ErrEmptyMessage))
}
