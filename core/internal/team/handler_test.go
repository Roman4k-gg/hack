package team

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ai-buddy/core/internal/onboarding"
	"ai-buddy/core/internal/platform/errors"
	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T) (*Handler, *fakeStore) {
	t.Helper()

	store := &fakeStore{snapshot: demoSnapshot()}
	service := NewService(store)
	service.now = func() time.Time { return day(0) }

	return NewHandler(service, nil, nil), store
}

func withPrincipal(req *http.Request, role string, scopes ...string) *http.Request {
	return req.WithContext(reqctx.WithPrincipal(req.Context(), reqctx.Principal{
		UserID: "mentor-1",
		Email:  "petr@demo.local",
		Role:   role,
		TeamID: "team-1",
		Scopes: scopes,
	}))
}

func TestHandleInternsReturnsReport(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/interns", nil), rbac.RoleMentor, rbac.ScopeTeamRead)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var report Report
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &report))
	require.Len(t, report.Interns, 2)
	require.Equal(t, 2, report.Overview.Interns)
	require.Equal(t, "Дарья Соколова", report.Interns[0].FullName)
}

func TestHandleInternsWithoutScopeIsForbidden(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/interns", nil), rbac.RoleIntern)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)

	var envelope errors.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, errors.CodeForbidden, envelope.Error.Code)
}

func TestHandleInternsWithoutPrincipalIsUnauthorized(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/interns", nil)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandleOverviewReturnsAggregates(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/overview", nil), rbac.RoleMentor, rbac.ScopeAnalyticsTeam)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var overview Overview
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &overview))
	require.Equal(t, 2, overview.Interns)
	require.Equal(t, 1, overview.AtRisk)
}

func TestHandleOverviewWithoutAnalyticsScopeIsForbidden(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/overview", nil), rbac.RoleMentor, rbac.ScopeTeamRead)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

type fakePlans struct {
	plan onboarding.Plan
	err  error
}

func (f *fakePlans) PlanForIntern(_ context.Context, _ reqctx.Principal, internID string) (onboarding.Plan, error) {
	if f.err != nil {
		return onboarding.Plan{}, f.err
	}

	if f.plan.ID == "" {
		return onboarding.Plan{}, onboarding.ErrPlanNotFound
	}

	return f.plan, nil
}

func TestHandleInternPlanReturnsPlan(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)
	handler.plans = &fakePlans{plan: onboarding.Plan{ID: "plan-1", Title: "Адаптация в команде Search"}}

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/interns/intern-1/plan", nil), rbac.RoleMentor, rbac.ScopeTeamRead)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var plan onboarding.Plan
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &plan))
	require.Equal(t, "Адаптация в команде Search", plan.Title)
}

func TestHandleInternPlanWithoutScopeIsForbidden(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)
	handler.plans = &fakePlans{plan: onboarding.Plan{ID: "plan-1"}}

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/interns/intern-1/plan", nil), rbac.RoleIntern)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandleInternPlanOfForeignInternIsForbidden(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)
	handler.plans = &fakePlans{err: onboarding.ErrForbidden}

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/interns/intern-1/plan", nil), rbac.RoleMentor, rbac.ScopeTeamRead)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandleInternPlanNotFound(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)
	handler.plans = &fakePlans{err: onboarding.ErrPlanNotFound}

	req := withPrincipal(httptest.NewRequest(http.MethodGet, "/interns/missing/plan", nil), rbac.RoleMentor, rbac.ScopeTeamRead)
	rec := httptest.NewRecorder()

	handler.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}
