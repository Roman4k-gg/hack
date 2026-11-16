package requests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-buddy/core/internal/integrations"
	"ai-buddy/core/internal/platform/errors"
	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/stretchr/testify/require"
)

type stubBundles struct {
	items []AccessItem
}

func (b *stubBundles) Bundle(_ context.Context, bundle string) ([]AccessItem, error) {
	if bundle == "unknown" {
		return nil, nil
	}

	return b.items, nil
}

func newAccessHandler(t *testing.T) (*Handler, *fakeStore, *fakeTokens) {
	t.Helper()

	store := newFakeStore()
	tokens := newFakeTokens()
	absence := &fakeAbsence{result: AbsenceResult{ExternalID: "ABS-1", Status: StatusPending}}
	bundles := &stubBundles{items: []AccessItem{
		{System: "VPN", Reason: "Доступ в сеть", Approver: "IT-отдел"},
		{System: "Jira", Reason: "Задачи команды", Approver: "Наставник"},
	}}

	service := NewService(store, tokens, absence, bundles, nil, nil)
	service.now = func() time.Time { return time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC) }

	return NewHandler(service, nil), store, tokens
}

func withPrincipalScopes(req *http.Request, scopes ...string) *http.Request {
	return req.WithContext(reqctx.WithPrincipal(req.Context(), reqctx.Principal{
		UserID: "user-1",
		Role:   rbac.RoleIntern,
		TeamID: "team-1",
		Scopes: scopes,
	}))
}

func TestHandleAccessPreviewReturnsBundle(t *testing.T) {
	t.Parallel()

	handler, _, _ := newAccessHandler(t)

	req := withPrincipalScopes(httptest.NewRequest(http.MethodPost, "/access/preview",
		strings.NewReader(`{"bundle":"intern"}`)), rbac.ScopeAccessSelf)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var preview AccessPreview
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))
	require.Len(t, preview.Items, 2)
	require.NotEmpty(t, preview.ConfirmationToken)
}

func TestHandleAccessPreviewWithBrokenBody(t *testing.T) {
	t.Parallel()

	handler, _, _ := newAccessHandler(t)

	req := withPrincipalScopes(httptest.NewRequest(http.MethodPost, "/access/preview",
		strings.NewReader(`{"bundle":`)), rbac.ScopeAccessSelf)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleAccessPreviewWithUnknownBundle(t *testing.T) {
	t.Parallel()

	handler, _, _ := newAccessHandler(t)

	req := withPrincipalScopes(httptest.NewRequest(http.MethodPost, "/access/preview",
		strings.NewReader(`{"bundle":"unknown"}`)), rbac.ScopeAccessSelf)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleAccessCreateWithoutScopeIsForbidden(t *testing.T) {
	t.Parallel()

	handler, _, _ := newAccessHandler(t)

	req := withPrincipalScopes(httptest.NewRequest(http.MethodPost, "/access",
		strings.NewReader(`{"preview_id":"p","confirmation_token":"t","idempotency_key":"k"}`)))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandleAccessCreateCreatesRequests(t *testing.T) {
	t.Parallel()

	handler, store, _ := newAccessHandler(t)

	preview := withPrincipalScopes(httptest.NewRequest(http.MethodPost, "/access/preview",
		strings.NewReader(`{"bundle":"intern"}`)), rbac.ScopeAccessSelf)
	rec := httptest.NewRecorder()
	handler.Router().ServeHTTP(rec, preview)

	var previewBody AccessPreview
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &previewBody))

	create := withPrincipalScopes(httptest.NewRequest(http.MethodPost, "/access",
		strings.NewReader(`{"preview_id":"`+previewBody.PreviewID+`","confirmation_token":"`+previewBody.ConfirmationToken+`","idempotency_key":"acc-key"}`)),
		rbac.ScopeAccessSelfWrite)
	rec = httptest.NewRecorder()
	handler.Router().ServeHTTP(rec, create)

	require.Equal(t, http.StatusOK, rec.Code)

	var result AccessResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Len(t, result.Requests, 2)
	require.NotEmpty(t, result.NextStep)
	require.Len(t, store.created, 2)
}

func TestHandleListReturnsStoredRequests(t *testing.T) {
	t.Parallel()

	handler, store, _ := newAccessHandler(t)

	store.records["request-1"] = Record{
		ID:        "request-1",
		Kind:      KindAbsence,
		Status:    StatusPending,
		Title:     "Отгул 20.11.2026",
		UserID:    "user-1",
		CreatedAt: time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC),
	}

	req := withPrincipalScopes(httptest.NewRequest(http.MethodGet, "/", nil), rbac.ScopeRequestSelf)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var list List
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list.Items, 1)
	require.Equal(t, "request-1", list.Items[0].RequestID)
}

func TestHandleListWithoutScopeIsForbidden(t *testing.T) {
	t.Parallel()

	handler, _, _ := newAccessHandler(t)

	req := withPrincipalScopes(httptest.NewRequest(http.MethodGet, "/", nil))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeForbidden, envelope.Error.Code)
}

func TestHandleGetForeignRequestIsForbidden(t *testing.T) {
	t.Parallel()

	handler, store, _ := newAccessHandler(t)

	store.records["request-1"] = Record{
		ID:        "request-1",
		Kind:      KindAbsence,
		Status:    StatusPending,
		Title:     "Чужой отгул",
		UserID:    "user-2",
		CreatedAt: time.Now(),
	}

	req := withPrincipalScopes(httptest.NewRequest(http.MethodGet, "/request-1", nil), rbac.ScopeRequestSelf)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandlePreviewWithoutPrincipalIsUnauthorized(t *testing.T) {
	t.Parallel()

	handler, _, _ := newAccessHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/absence/preview",
		strings.NewReader(`{"date_from":"2026-11-20","date_to":"2026-11-20","absence_type":"day_off"}`))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAbsenceAdapterWrapsIntegrations(t *testing.T) {
	t.Parallel()

	mock := integrations.NewMockAbsence(0)
	adapter := NewAbsenceAdapter(mock)

	notes, err := adapter.Policy(context.Background(), AbsenceInput{AbsenceType: "day_off"})
	require.NoError(t, err)
	require.NotEmpty(t, notes)

	result, err := adapter.Create(context.Background(), AbsenceInput{AbsenceType: "day_off"})
	require.NoError(t, err)
	require.NotEmpty(t, result.ExternalID)
}

func TestAbsenceAdapterWithoutService(t *testing.T) {
	t.Parallel()

	adapter := NewAbsenceAdapter(nil)

	notes, err := adapter.Policy(context.Background(), AbsenceInput{})
	require.NoError(t, err)
	require.Empty(t, notes)

	result, err := adapter.Create(context.Background(), AbsenceInput{})
	require.NoError(t, err)
	require.Equal(t, StatusPending, result.Status)
}
