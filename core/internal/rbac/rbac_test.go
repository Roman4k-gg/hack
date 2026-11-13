package rbac

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ai-buddy/core/internal/platform/errors"
	"ai-buddy/core/internal/platform/reqctx"

	"github.com/stretchr/testify/require"
)

func TestScopesForRole(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{ScopeAll}, ScopesForRole(RoleAdmin))
	require.Contains(t, ScopesForRole(RoleIntern), ScopePlanRead)
	require.Contains(t, ScopesForRole(RoleMentor), ScopeChat)
	require.Empty(t, ScopesForRole("unknown"))
	require.Empty(t, ScopesForRole(""))
}

func TestHasScope(t *testing.T) {
	t.Parallel()

	require.True(t, HasScope([]string{ScopeAll}, ScopeChat))
	require.True(t, HasScope([]string{ScopePlanRead}, ScopePlanRead))
	require.False(t, HasScope([]string{ScopePlanRead}, ScopeChat))
	require.False(t, HasScope(nil, ScopeChat))
}

func TestHasAllScopes(t *testing.T) {
	t.Parallel()

	require.True(t, HasAllScopes([]string{ScopeAll}, []string{ScopePlanRead, ScopeChat}))
	require.True(t, HasAllScopes([]string{ScopePlanRead, ScopeChat}, []string{ScopePlanRead}))
	require.False(t, HasAllScopes([]string{ScopePlanRead}, []string{ScopePlanRead, ScopeChat}))
}

func TestRequireScopesAllowsRequestWithScope(t *testing.T) {
	t.Parallel()

	handler := RequireScopes(ScopePlanRead)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/plan", nil)
	req = req.WithContext(reqctx.WithPrincipal(req.Context(), reqctx.Principal{
		UserID: "user-1",
		Role:   RoleIntern,
		Scopes: []string{ScopePlanRead},
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireScopesRejectsRequestWithoutScope(t *testing.T) {
	t.Parallel()

	handler := RequireScopes(ScopePlanWrite)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/plan", nil)
	req = req.WithContext(reqctx.WithPrincipal(req.Context(), reqctx.Principal{
		UserID: "user-1",
		Role:   RoleIntern,
		Scopes: []string{ScopePlanRead},
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeForbidden, envelope.Error.Code)
	require.Equal(t, ScopePlanWrite, envelope.Error.Details["scope"])
}

func TestRequireScopesRejectsAnonymousRequest(t *testing.T) {
	t.Parallel()

	handler := RequireScopes(ScopePlanRead)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/plan", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
