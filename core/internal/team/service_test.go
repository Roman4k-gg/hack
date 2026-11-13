package team

import (
	"context"
	"testing"
	"time"

	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	snapshot Snapshot
	err      error
	filter   Filter
}

func (s *fakeStore) Snapshot(_ context.Context, filter Filter) (Snapshot, error) {
	s.filter = filter

	if s.err != nil {
		return Snapshot{}, s.err
	}

	return s.snapshot, nil
}

func day(offset int) time.Time {
	return time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC).AddDate(0, 0, offset)
}

func demoSnapshot() Snapshot {
	last := day(0).Add(9 * time.Hour)

	return Snapshot{
		Interns: []InternRecord{
			{
				ID: "intern-1", FullName: "Дарья Соколова", Email: "dasha@demo.local",
				Position: "Стажёр-разработчик",
				TeamID:   "team-1", TeamName: "Search", TeamSlug: "search",
				MentorID: "mentor-1", MentorName: "Пётр Иванов",
				PlanID: "plan-1", PlanTitle: "Адаптация в команде Search",
				StartsOn: day(-3), HasPlan: true,
			},
			{
				ID: "intern-2", FullName: "Мария Кузнецова", Email: "masha@demo.local",
				Position: "Стажёр-разработчик",
				TeamID:   "team-2", TeamName: "Ads", TeamSlug: "ads",
				PlanID: "plan-2", PlanTitle: "Адаптация в команде Ads",
				StartsOn: day(-10), HasPlan: true,
			},
		},
		Items: []ItemRecord{
			{InternID: "intern-1", ItemID: "item-1", StageTitle: "Неделя 1 · Окружение", StagePosition: 1, Position: 1, Title: "Ноутбук и VPN", Status: "done", DueOn: ptr(day(-2))},
			{InternID: "intern-1", ItemID: "item-2", StageTitle: "Неделя 1 · Окружение", StagePosition: 1, Position: 2, Title: "Доступ к Jira", Status: "in_progress", DueOn: ptr(day(1))},
			{InternID: "intern-1", ItemID: "item-3", StageTitle: "Неделя 2 · Процессы", StagePosition: 2, Position: 1, Title: "Курс по безопасности", Status: "blocked", BlockedReason: "Ждём подтверждения от HR", DueOn: ptr(day(-1))},
			{InternID: "intern-1", ItemID: "item-4", StageTitle: "Неделя 2 · Процессы", StagePosition: 2, Position: 2, Title: "Первый PR", Status: "pending", DueOn: ptr(day(5))},
			{InternID: "intern-2", ItemID: "item-5", StageTitle: "Неделя 1 · Окружение", StagePosition: 1, Position: 1, Title: "Ноутбук и VPN", Status: "done", DueOn: ptr(day(-8))},
			{InternID: "intern-2", ItemID: "item-6", StageTitle: "Неделя 1 · Окружение", StagePosition: 1, Position: 2, Title: "Доступ к Jira", Status: "done", DueOn: ptr(day(-2))},
			{InternID: "intern-2", ItemID: "item-7", StageTitle: "Неделя 2 · Процессы", StagePosition: 2, Position: 1, Title: "Курс по безопасности", Status: "skipped", DueOn: ptr(day(-1))},
		},
		Activity: map[string]ActivityRecord{
			"intern-1": {UserID: "intern-1", Questions: 3, LastEventAt: &last},
		},
	}
}

func ptr(value time.Time) *time.Time {
	return &value
}

func TestReportComputesMemberMetrics(t *testing.T) {
	t.Parallel()

	store := &fakeStore{snapshot: demoSnapshot()}
	service := NewService(store)
	service.now = func() time.Time { return day(0) }

	report, err := service.Report(context.Background(), reqctx.Principal{
		UserID: "mentor-1", Role: rbac.RoleMentor, TeamID: "team-1",
		Scopes: []string{rbac.ScopeTeamRead},
	})
	require.NoError(t, err)
	require.Len(t, report.Interns, 2)

	dasha := report.Interns[0]
	require.Equal(t, "Дарья Соколова", dasha.FullName)
	require.Equal(t, "Search", dasha.Team.Name)
	require.NotNil(t, dasha.Mentor)
	require.Equal(t, "Пётр Иванов", dasha.Mentor.FullName)
	require.Equal(t, 4, dasha.Progress.Total)
	require.Equal(t, 1, dasha.Progress.Done)
	require.Equal(t, 1, dasha.Progress.InProgress)
	require.Equal(t, 1, dasha.Progress.Blocked)
	require.Equal(t, 1, dasha.Progress.Pending)
	require.Equal(t, 25, dasha.Progress.Percent)
	require.Equal(t, "Неделя 1 · Окружение", dasha.Plan.CurrentStage)
	require.Len(t, dasha.Blockers, 1)
	require.Equal(t, "Ждём подтверждения от HR", dasha.Blockers[0].Reason)
	require.Len(t, dasha.Overdue, 1)
	require.Equal(t, 1, dasha.Overdue[0].DaysOverdue)
	require.True(t, dasha.AtRisk)
	require.Equal(t, 3, dasha.Activity.Questions)
	require.NotNil(t, dasha.Activity.LastEventAt)

	masha := report.Interns[1]
	require.Equal(t, 66, masha.Progress.Percent)
	require.Equal(t, 66, masha.Pace.Percent)
	require.Empty(t, masha.Blockers)
	require.Empty(t, masha.Overdue)
	require.False(t, masha.AtRisk)
	require.Equal(t, 0, masha.Activity.Questions)
}

func TestReportAggregatesOverview(t *testing.T) {
	t.Parallel()

	store := &fakeStore{snapshot: demoSnapshot()}
	service := NewService(store)
	service.now = func() time.Time { return day(0) }

	report, err := service.Report(context.Background(), reqctx.Principal{
		UserID: "admin-1", Role: rbac.RoleAdmin, Scopes: []string{rbac.ScopeAll},
	})
	require.NoError(t, err)

	overview := report.Overview
	require.Equal(t, 2, overview.Interns)
	require.Equal(t, 1, overview.Blocked)
	require.Equal(t, 1, overview.Overdue)
	require.Equal(t, 1, overview.AtRisk)
	require.Len(t, overview.Teams, 2)
}

func TestReportFiltersByTeamForMentor(t *testing.T) {
	t.Parallel()

	store := &fakeStore{snapshot: demoSnapshot()}
	service := NewService(store)

	_, err := service.Report(context.Background(), reqctx.Principal{
		UserID: "mentor-1", Role: rbac.RoleMentor, TeamID: "team-1",
		Scopes: []string{rbac.ScopeTeamRead},
	})
	require.NoError(t, err)
	require.False(t, store.filter.All)
	require.Equal(t, "team-1", store.filter.TeamID)
	require.Equal(t, "mentor-1", store.filter.MentorID)
}

func TestReportAllowsEverythingForAdmin(t *testing.T) {
	t.Parallel()

	store := &fakeStore{snapshot: demoSnapshot()}
	service := NewService(store)

	_, err := service.Report(context.Background(), reqctx.Principal{
		UserID: "admin-1", Role: rbac.RoleAdmin, Scopes: []string{rbac.ScopeAll},
	})
	require.NoError(t, err)
	require.True(t, store.filter.All)
}

func TestReportFailsWithoutStore(t *testing.T) {
	t.Parallel()

	service := NewService(nil)

	_, err := service.Report(context.Background(), reqctx.Principal{})
	require.Error(t, err)
}
