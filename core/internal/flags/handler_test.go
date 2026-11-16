package flags

import (
	"context"
	"encoding/json"
	stderrs "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apierrors "ai-buddy/core/internal/platform/errors"
	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/stretchr/testify/require"
)

type handlerStore struct {
	flags   map[string]Flag
	failing bool
}

func newHandlerStore() *handlerStore {
	return &handlerStore{flags: map[string]Flag{
		"kb_citations": {
			Key:         "kb_citations",
			Enabled:     true,
			Rollout:     100,
			Description: "Цитаты в ответах",
			UpdatedAt:   time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC),
		},
	}}
}

func (s *handlerStore) List(_ context.Context) ([]Flag, error) {
	if s.failing {
		return nil, stderrs.New("db is down")
	}

	flags := []Flag{}
	for _, flag := range s.flags {
		flags = append(flags, flag)
	}

	return flags, nil
}

func (s *handlerStore) Set(_ context.Context, key string, enabled bool, rollout int, _ string) (Flag, error) {
	if s.failing {
		return Flag{}, stderrs.New("db is down")
	}

	flag := s.flags[key]
	flag.Key = key
	flag.Enabled = enabled
	flag.Rollout = rollout

	if flag.Description == "" {
		flag.Description = "Флаг без описания"
	}

	s.flags[key] = flag

	return flag, nil
}

func (s *handlerStore) Get(_ context.Context, key string) (Flag, error) {
	if s.failing {
		return Flag{}, stderrs.New("db is down")
	}

	flag, ok := s.flags[key]
	if !ok {
		return Flag{}, ErrFlagNotFound
	}

	return flag, nil
}

func newTestHandler(t *testing.T) (*Handler, *handlerStore) {
	t.Helper()

	store := newHandlerStore()

	return NewHandler(NewService(store), nil), store
}

func withPrincipal(req *http.Request, scopes ...string) *http.Request {
	return req.WithContext(reqctx.WithPrincipal(req.Context(), reqctx.Principal{
		UserID: "admin-1",
		Role:   rbac.RoleAdmin,
		Scopes: scopes,
	}))
}

func TestHandleListReturnsFlags(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/", nil), rbac.ScopeAdminFlags)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var flags []Flag
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &flags))
	require.Len(t, flags, 1)
	require.Equal(t, "kb_citations", flags[0].Key)
}

func TestHandleListWithoutScopeIsForbidden(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)

	var envelope apierrors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, apierrors.CodeForbidden, envelope.Error.Code)
}

func TestHandleListFails(t *testing.T) {
	t.Parallel()

	handler, store := newTestHandler(t)
	store.failing = true

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/", nil), rbac.ScopeAdminFlags)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleUpdateDisablesFlag(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodPatch, "/kb_citations", strings.NewReader(`{"enabled":false}`)),
		rbac.ScopeAdminFlags)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var flag Flag
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &flag))
	require.False(t, flag.Enabled)
}

func TestHandleUpdateKeepsUnsetFields(t *testing.T) {
	t.Parallel()

	handler, store := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodPatch, "/kb_citations", strings.NewReader(`{"rollout":40}`)),
		rbac.ScopeAdminFlags)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	require.True(t, store.flags["kb_citations"].Enabled)
	require.Equal(t, 40, store.flags["kb_citations"].Rollout)
}

func TestHandleUpdateCreatesUnknownFlag(t *testing.T) {
	t.Parallel()

	handler, store := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodPatch, "/new_flag", strings.NewReader(`{"enabled":true}`)),
		rbac.ScopeAdminFlags)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, store.flags["new_flag"].Enabled)
}

func TestHandleUpdateWithBrokenBody(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodPatch, "/kb_citations", strings.NewReader(`{"enabled":`)),
		rbac.ScopeAdminFlags)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleUpdateWithBrokenRollout(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodPatch, "/kb_citations", strings.NewReader(`{"rollout":500}`)),
		rbac.ScopeAdminFlags)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var envelope apierrors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, apierrors.CodeValidation, envelope.Error.Code)
}

func TestHandleUpdateWithoutScopeIsForbidden(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodPatch, "/kb_citations", strings.NewReader(`{"enabled":false}`)))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandleUpdateWithoutPrincipalIsUnauthorized(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPatch, "/kb_citations", strings.NewReader(`{"enabled":false}`))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
