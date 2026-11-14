package requests

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

func newTestHandler(t *testing.T) (*Handler, *fakeStore, *fakeTokens) {
	t.Helper()

	store := newFakeStore()
	tokens := newFakeTokens()
	absence := &fakeAbsence{
		notes:  []string{"Отгул согласует руководитель"},
		result: AbsenceResult{ExternalID: "ABS-1024", Status: StatusPending},
	}

	service := NewService(store, tokens, absence, nil, nil, nil)

	return NewHandler(service, nil), store, tokens
}

func withPrincipal(req *http.Request, scopes ...string) *http.Request {
	return req.WithContext(reqctx.WithPrincipal(req.Context(), reqctx.Principal{
		UserID: "user-1",
		Email:  "dasha@demo.local",
		Role:   rbac.RoleIntern,
		TeamID: "team-1",
		Scopes: scopes,
	}))
}

func TestHandlePreviewReturnsToken(t *testing.T) {
	t.Parallel()

	handler, _, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodPost, "/absence/preview",
		jsonBody(`{"date_from":"2026-11-20","date_to":"2026-11-20","absence_type":"day_off"}`)),
		rbac.ScopeAbsenceSelf)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var preview Preview
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))
	require.NotEmpty(t, preview.ConfirmationToken)
	require.Equal(t, "day_off", preview.Draft.AbsenceType)
}

func TestHandlePreviewWithoutScopeIsForbidden(t *testing.T) {
	t.Parallel()

	handler, _, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodPost, "/absence/preview",
		jsonBody(`{"date_from":"2026-11-20","date_to":"2026-11-20","absence_type":"day_off"}`)))
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeForbidden, envelope.Error.Code)
}

func TestHandlePreviewWithBadDatesIsBadRequest(t *testing.T) {
	t.Parallel()

	handler, _, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodPost, "/absence/preview",
		jsonBody(`{"date_from":"2026-11-20","date_to":"2026-11-19","absence_type":"day_off"}`)),
		rbac.ScopeAbsenceSelf)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleCreateWithoutTokenIsUnauthorized(t *testing.T) {
	t.Parallel()

	handler, _, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodPost, "/absence",
		jsonBody(`{"preview_id":"preview-1","confirmation_token":"","idempotency_key":"key-1"}`)),
		rbac.ScopeAbsenceSelfWrite)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandleCreateReturnsRequest(t *testing.T) {
	t.Parallel()

	handler, _, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodPost, "/absence/preview",
		jsonBody(`{"date_from":"2026-11-20","date_to":"2026-11-20","absence_type":"day_off"}`)),
		rbac.ScopeAbsenceSelf)
	rec := httptest.NewRecorder()
	handler.Router().ServeHTTP(rec, req)

	var preview Preview
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))

	req = withPrincipal(httptest.NewRequest(http.MethodPost, "/absence",
		jsonBody(`{"preview_id":"`+preview.PreviewID+`","confirmation_token":"`+preview.ConfirmationToken+`","idempotency_key":"key-1"}`)),
		rbac.ScopeAbsenceSelfWrite)
	rec = httptest.NewRecorder()
	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var detail Detail
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &detail))
	require.Equal(t, StatusPending, detail.Status)
	require.Equal(t, "ABS-1024", detail.ExternalID)
	require.Len(t, detail.History, 1)
}

func TestHandleListReturnsItems(t *testing.T) {
	t.Parallel()

	handler, _, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/", nil), rbac.ScopeRequestSelf)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var list List
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Empty(t, list.Items)
}

func TestHandleGetNotFound(t *testing.T) {
	t.Parallel()

	handler, _, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/missing", nil), rbac.ScopeRequestSelf)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleWithoutPrincipalIsUnauthorized(t *testing.T) {
	t.Parallel()

	handler, _, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func jsonBody(payload string) *strings.Reader {
	return strings.NewReader(payload)
}
