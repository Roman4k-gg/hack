package onboarding

import (
	"context"
	"testing"
	"time"

	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/stretchr/testify/require"
)

type fakePlanStore struct {
	plan      Plan
	items     map[string]Item
	updateErr error
}

func newFakePlanStore() *fakePlanStore {
	return &fakePlanStore{items: map[string]Item{}}
}

func (s *fakePlanStore) PlanByUserID(_ context.Context, userID string) (Plan, error) {
	if s.plan.ID == "" {
		return Plan{}, ErrPlanNotFound
	}
	return s.plan, nil
}

func (s *fakePlanStore) ItemByID(_ context.Context, itemID string) (Item, error) {
	item, ok := s.items[itemID]
	if !ok {
		return Item{}, ErrItemNotFound
	}
	return item, nil
}

func (s *fakePlanStore) UpdateItemStatus(_ context.Context, itemID, status, blockedReason string) (Item, error) {
	if s.updateErr != nil {
		return Item{}, s.updateErr
	}
	item, ok := s.items[itemID]
	if !ok {
		return Item{}, ErrItemNotFound
	}
	item.Status = status
	item.BlockedReason = blockedReason
	if status == "done" {
		now := time.Now()
		item.CompletedAt = &now
	} else {
		item.CompletedAt = nil
	}
	s.items[itemID] = item
	return item, nil
}

func demoPlan() Plan {
	return Plan{
		ID:       "plan-1",
		Title:    "Адаптация в команде Search",
		StartsOn: time.Now().AddDate(0, 0, -3),
		Stages: []Stage{
			{
				ID:       "stage-1",
				Title:    "Неделя 1 · Окружение",
				Position: 1,
				Items: []Item{
					{ID: "item-1", StageID: "stage-1", Title: "Ноутбук и VPN", Status: "done", Position: 1},
					{ID: "item-2", StageID: "stage-1", Title: "Доступ к Jira", Status: "in_progress", Position: 2},
				},
				Progress: Progress{Done: 1, Total: 2, Percent: 50},
			},
			{
				ID:       "stage-2",
				Title:    "Неделя 2 · Процессы",
				Position: 2,
				Items: []Item{
					{ID: "item-3", StageID: "stage-2", Title: "Курс по безопасности", Status: "blocked", BlockedReason: "Ждём HR", Position: 1},
				},
				Progress: Progress{Done: 0, Total: 1, Percent: 0},
			},
		},
		Progress: Progress{Done: 1, Total: 3, Percent: 33},
		NextStep: &Item{ID: "item-2", StageID: "stage-1", Title: "Доступ к Jira", Status: "in_progress"},
	}
}

func TestProgressAndNextStep(t *testing.T) {
	t.Parallel()

	items := []Item{
		{Status: "done"},
		{Status: "done"},
		{Status: "pending"},
	}

	progress := progressOf(items)
	require.Equal(t, 2, progress.Done)
	require.Equal(t, 3, progress.Total)
	require.Equal(t, 66, progress.Percent)

	require.Equal(t, Progress{}, progressOf(nil))
}

func TestNextStepSkipsDoneAndSkipped(t *testing.T) {
	t.Parallel()

	stages := []Stage{
		{Items: []Item{{ID: "a", Status: "done"}, {ID: "b", Status: "skipped"}}},
		{Items: []Item{{ID: "c", Status: "pending"}}},
	}

	next := nextStepOf(stages)
	require.NotNil(t, next)
	require.Equal(t, "c", next.ID)

	require.Nil(t, nextStepOf([]Stage{{Items: []Item{{ID: "a", Status: "done"}}}}))
}

func TestServicePlanReturnsPlan(t *testing.T) {
	t.Parallel()

	store := newFakePlanStore()
	store.plan = demoPlan()

	service := NewService(store, nil, nil)

	plan, err := service.Plan(context.Background(), "user-1")
	require.NoError(t, err)
	require.Equal(t, "Адаптация в команде Search", plan.Title)
	require.Len(t, plan.Stages, 2)
	require.Equal(t, 33, plan.Progress.Percent)
	require.NotNil(t, plan.NextStep)
	require.Equal(t, "Доступ к Jira", plan.NextStep.Title)

	_, err = service.Plan(context.Background(), "")
	require.ErrorIs(t, err, ErrPlanNotFound)
}

func TestServiceUpdateItemValidatesStatus(t *testing.T) {
	t.Parallel()

	store := newFakePlanStore()
	store.items["item-1"] = Item{ID: "item-1", OwnerID: "user-1", Status: "pending"}

	service := NewService(store, nil, nil)
	principal := reqctx.Principal{UserID: "user-1", Role: rbac.RoleIntern}

	_, err := service.UpdateItem(context.Background(), principal, "item-1", UpdateItemInput{Status: "unknown"})
	require.ErrorIs(t, err, ErrInvalidStatus)

	_, err = service.UpdateItem(context.Background(), principal, "item-1", UpdateItemInput{Status: "blocked"})
	require.ErrorIs(t, err, ErrMissingReason)

	_, err = service.UpdateItem(context.Background(), principal, "missing", UpdateItemInput{Status: "done"})
	require.ErrorIs(t, err, ErrItemNotFound)
}

func TestServiceUpdateItemChecksOwnership(t *testing.T) {
	t.Parallel()

	store := newFakePlanStore()
	store.items["item-1"] = Item{ID: "item-1", OwnerID: "user-1", Status: "pending"}

	service := NewService(store, nil, nil)

	_, err := service.UpdateItem(context.Background(),
		reqctx.Principal{UserID: "user-2", Role: rbac.RoleIntern},
		"item-1", UpdateItemInput{Status: "done"})
	require.ErrorIs(t, err, ErrForbidden)

	_, err = service.UpdateItem(context.Background(),
		reqctx.Principal{UserID: "user-2", Role: rbac.RoleMentor},
		"item-1", UpdateItemInput{Status: "done"})
	require.NoError(t, err)
}

func TestServiceUpdateItemUpdatesStatus(t *testing.T) {
	t.Parallel()

	store := newFakePlanStore()
	store.items["item-1"] = Item{ID: "item-1", OwnerID: "user-1", Status: "pending"}

	service := NewService(store, nil, nil)
	principal := reqctx.Principal{UserID: "user-1", Role: rbac.RoleIntern}

	item, err := service.UpdateItem(context.Background(), principal, "item-1", UpdateItemInput{Status: "done"})
	require.NoError(t, err)
	require.Equal(t, "done", item.Status)
	require.NotNil(t, item.CompletedAt)

	item, err = service.UpdateItem(context.Background(), principal, "item-1",
		UpdateItemInput{Status: "blocked", BlockedReason: "Ждём HR"})
	require.NoError(t, err)
	require.Equal(t, "blocked", item.Status)
	require.Equal(t, "Ждём HR", item.BlockedReason)
	require.Nil(t, item.CompletedAt)

	item, err = service.UpdateItem(context.Background(), principal, "item-1",
		UpdateItemInput{Status: "in_progress", BlockedReason: "Ждём HR"})
	require.NoError(t, err)
	require.Empty(t, item.BlockedReason)
}

func TestNormalizeStatus(t *testing.T) {
	t.Parallel()

	require.Equal(t, "done", normalizeStatus(" DONE "))
	require.Equal(t, "in_progress", normalizeStatus("in_progress"))
	require.Equal(t, "", normalizeStatus("unknown"))
}
