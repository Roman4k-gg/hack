package onboarding

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ai-buddy/core/internal/platform/errors"
	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T) (*Handler, *fakePlanStore) {
	t.Helper()

	store := newFakePlanStore()
	store.plan = demoPlan()
	store.items["item-2"] = Item{ID: "item-2", OwnerID: "user-1", Status: "in_progress"}

	service := NewService(store, nil, nil)
	return NewHandler(service, nil), store
}

func withPrincipal(req *http.Request, scopes ...string) *http.Request {
	return req.WithContext(reqctx.WithPrincipal(req.Context(), reqctx.Principal{
		UserID: "user-1",
		Email:  "dasha@demo.local",
		Role:   rbac.RoleIntern,
		Scopes: scopes,
	}))
}

func TestHandlePlanReturnsPlan(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(
		httptest.NewRequest(http.MethodGet, "/plan", nil),
		rbac.ScopePlanRead,
	)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var plan Plan
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &plan))
	require.Equal(t, "Адаптация в команде Search", plan.Title)
	require.Len(t, plan.Stages, 2)
	require.NotNil(t, plan.NextStep)
}

func TestHandlePlanWithoutScopeIsForbidden(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/plan", nil))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeForbidden, envelope.Error.Code)
}

func TestHandlePlanWithoutPrincipalIsUnauthorized(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/plan", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandleUpdateItemReturnsItem(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(
		httptest.NewRequest(http.MethodPatch, "/items/item-2", strings.NewReader(`{"status":"done"}`)),
		rbac.ScopePlanWrite,
	)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var item Item
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &item))
	require.Equal(t, "done", item.Status)
}

func TestHandleUpdateItemWithInvalidStatus(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(
		httptest.NewRequest(http.MethodPatch, "/items/item-2", strings.NewReader(`{"status":"whatever"}`)),
		rbac.ScopePlanWrite,
	)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeValidation, envelope.Error.Code)
}

func TestHandleUpdateItemWithUnknownItem(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(
		httptest.NewRequest(http.MethodPatch, "/items/missing", strings.NewReader(`{"status":"done"}`)),
		rbac.ScopePlanWrite,
	)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeNotFound, envelope.Error.Code)
}
