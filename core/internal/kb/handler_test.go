package kb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ai-buddy/core/internal/platform/errors"
	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T) (*Handler, *fakeStore) {
	t.Helper()

	store := &fakeStore{
		items: []Item{{
			DocumentID: "doc-1",
			Title:      "Как получить доступ к Jira",
			Team:       "Search",
			Score:      1,
			UpdatedAt:  time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC),
		}},
		document: Document{
			DocumentID: "doc-1",
			Title:      "Как получить доступ к Jira",
			Visibility: "team",
			TeamID:     "team-search",
			UpdatedAt:  time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC),
		},
	}

	service := NewService(store)
	service.now = func() time.Time { return time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC) }

	return NewHandler(service, nil), store
}

func withPrincipal(req *http.Request, scopes ...string) *http.Request {
	return req.WithContext(reqctx.WithPrincipal(req.Context(), reqctx.Principal{
		UserID: "user-1",
		Role:   rbac.RoleIntern,
		TeamID: "team-search",
		Scopes: scopes,
	}))
}

func TestHandleSearchReturnsItems(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/search?q=jira", nil),
		rbac.ScopeKBRead, rbac.ScopeKBReadTeam)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var result SearchResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Equal(t, 1, result.Total)
	require.Equal(t, "Как получить доступ к Jira", result.Items[0].Title)
}

func TestHandleSearchWithoutScopeIsForbidden(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/search?q=jira", nil))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeForbidden, envelope.Error.Code)
}

func TestHandleSearchWithoutPrincipalIsUnauthorized(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/search?q=jira", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandleSearchWithoutQueryIsBadRequest(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/search", nil),
		rbac.ScopeKBRead, rbac.ScopeKBReadTeam)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleSearchWithBrokenLimitIsBadRequest(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/search?q=jira&limit=many", nil),
		rbac.ScopeKBRead, rbac.ScopeKBReadTeam)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleDocumentReturnsDocument(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/documents/doc-1", nil),
		rbac.ScopeKBRead, rbac.ScopeKBReadTeam)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var document Document
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &document))
	require.Equal(t, "Как получить доступ к Jira", document.Title)
}

func TestHandleDocumentNotFound(t *testing.T) {
	t.Parallel()

	handler, store := newTestHandler(t)
	store.err = ErrDocumentNotFound

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/documents/missing", nil),
		rbac.ScopeKBRead, rbac.ScopeKBReadTeam)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}
