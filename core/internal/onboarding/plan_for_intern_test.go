package onboarding

import (
	"context"
	"testing"

	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/stretchr/testify/require"
)

func mentorPrincipal() reqctx.Principal {
	return reqctx.Principal{
		UserID: "mentor-1",
		Role:   rbac.RoleMentor,
		TeamID: "team-1",
		Scopes: []string{rbac.ScopeTeamRead},
	}
}

func TestPlanForInternReturnsPlanForMentor(t *testing.T) {
	t.Parallel()

	service := NewService(planStore(), nil, nil)

	plan, err := service.PlanForIntern(context.Background(), mentorPrincipal(), "intern-1")
	require.NoError(t, err)
	require.Equal(t, "Адаптация в команде Search", plan.Title)
	require.NotEmpty(t, plan.Stages)
}

func TestPlanForInternRejectsForeignIntern(t *testing.T) {
	t.Parallel()

	service := NewService(planStore(), nil, nil)

	other := mentorPrincipal()
	other.UserID = "mentor-9"
	other.TeamID = "team-9"

	_, err := service.PlanForIntern(context.Background(), other, "intern-1")
	require.ErrorIs(t, err, ErrForbidden)
}

func TestPlanForInternAllowsHR(t *testing.T) {
	t.Parallel()

	service := NewService(planStore(), nil, nil)

	hr := reqctx.Principal{
		UserID: "hr-1",
		Role:   rbac.RoleHR,
		Scopes: []string{rbac.ScopeTeamRead},
	}

	plan, err := service.PlanForIntern(context.Background(), hr, "intern-1")
	require.NoError(t, err)
	require.NotEmpty(t, plan.Stages)
}

func TestPlanForInternAllowsAdmin(t *testing.T) {
	t.Parallel()

	service := NewService(planStore(), nil, nil)

	admin := reqctx.Principal{
		UserID: "admin-1",
		Role:   rbac.RoleAdmin,
		Scopes: []string{rbac.ScopeAll},
	}

	plan, err := service.PlanForIntern(context.Background(), admin, "intern-1")
	require.NoError(t, err)
	require.NotEmpty(t, plan.Stages)
}

func TestPlanForInternRequiresTeamScope(t *testing.T) {
	t.Parallel()

	service := NewService(planStore(), nil, nil)

	intern := reqctx.Principal{
		UserID: "intern-1",
		Role:   rbac.RoleIntern,
		TeamID: "team-1",
		Scopes: []string{rbac.ScopePlanRead},
	}

	_, err := service.PlanForIntern(context.Background(), intern, "intern-1")
	require.ErrorIs(t, err, ErrForbidden)
}

func TestPlanForInternRejectsEmptyID(t *testing.T) {
	t.Parallel()

	service := NewService(planStore(), nil, nil)

	_, err := service.PlanForIntern(context.Background(), mentorPrincipal(), "  ")
	require.ErrorIs(t, err, ErrPlanNotFound)
}

func TestCanViewIntern(t *testing.T) {
	t.Parallel()

	access := InternAccess{UserID: "intern-1", TeamID: "team-1", MentorID: "mentor-1"}

	require.True(t, canViewIntern(mentorPrincipal(), access))
	require.False(t, canViewIntern(mentorPrincipal(), InternAccess{UserID: "intern-2", TeamID: "team-9"}))
	require.True(t, canViewIntern(reqctx.Principal{UserID: "hr-1", Role: rbac.RoleHR, Scopes: []string{rbac.ScopeTeamRead}}, access))
	require.True(t, canViewIntern(reqctx.Principal{UserID: "admin-1", Role: rbac.RoleAdmin, Scopes: []string{rbac.ScopeAll}}, access))
	require.False(t, canViewIntern(reqctx.Principal{UserID: "intern-1", Role: rbac.RoleIntern, Scopes: []string{rbac.ScopePlanRead}}, access))
}

func TestPlanForInternUsesStoreAccess(t *testing.T) {
	t.Parallel()

	store := planStore()
	service := NewService(store, nil, nil)

	plan, err := service.PlanForIntern(context.Background(), mentorPrincipal(), "intern-1")
	require.NoError(t, err)
	require.Equal(t, "Адаптация в команде Search", plan.Title)
}

func planStore() *fakePlanStore {
	store := newFakePlanStore()
	store.plan = demoPlan()

	return store
}
