package onboarding

import (
	"context"
	"testing"
	"time"

	"ai-buddy/core/internal/platform/reqctx"
	"ai-buddy/core/internal/rbac"

	"github.com/stretchr/testify/require"
)

func TestUpdateItemIsIdempotent(t *testing.T) {
	t.Parallel()

	store := newFakePlanStore()
	store.items["item-2"] = Item{ID: "item-2", OwnerID: "user-1", Status: "in_progress", Title: "Доступ к Jira"}

	service := NewService(store, nil, nil)

	principal := reqctx.Principal{
		UserID: "user-1",
		Role:   rbac.RoleIntern,
		Scopes: []string{rbac.ScopePlanWrite},
	}

	item, err := service.UpdateItem(context.Background(), principal, "item-2", UpdateItemInput{Status: "in_progress"})
	require.NoError(t, err)
	require.Equal(t, "in_progress", item.Status)
	require.Equal(t, 0, store.updates, "повтор того же статуса не должен писать в базу")
}

func TestUpdateItemWritesNewStatus(t *testing.T) {
	t.Parallel()

	store := newFakePlanStore()
	store.items["item-2"] = Item{ID: "item-2", OwnerID: "user-1", Status: "in_progress", Title: "Доступ к Jira"}

	service := NewService(store, nil, nil)

	principal := reqctx.Principal{
		UserID: "user-1",
		Role:   rbac.RoleIntern,
		Scopes: []string{rbac.ScopePlanWrite},
	}

	item, err := service.UpdateItem(context.Background(), principal, "item-2", UpdateItemInput{Status: "done"})
	require.NoError(t, err)
	require.Equal(t, "done", item.Status)
	require.Equal(t, 1, store.updates)
}

func TestUpdateItemValidatesStatus(t *testing.T) {
	t.Parallel()

	store := newFakePlanStore()
	store.items["item-2"] = Item{ID: "item-2", OwnerID: "user-1", Status: "in_progress"}

	service := NewService(store, nil, nil)

	principal := reqctx.Principal{UserID: "user-1", Role: rbac.RoleIntern}

	_, err := service.UpdateItem(context.Background(), principal, "item-2", UpdateItemInput{Status: "unknown"})
	require.ErrorIs(t, err, ErrInvalidStatus)

	_, err = service.UpdateItem(context.Background(), principal, "item-2", UpdateItemInput{Status: "blocked"})
	require.ErrorIs(t, err, ErrMissingReason)
}

func TestUpdateItemRejectsForeignItem(t *testing.T) {
	t.Parallel()

	store := newFakePlanStore()
	store.items["item-2"] = Item{ID: "item-2", OwnerID: "user-2", Status: "in_progress"}

	service := NewService(store, nil, nil)

	principal := reqctx.Principal{UserID: "user-1", Role: rbac.RoleIntern}

	_, err := service.UpdateItem(context.Background(), principal, "item-2", UpdateItemInput{Status: "done"})
	require.ErrorIs(t, err, ErrForbidden)
}

func TestUpdateItemNotFound(t *testing.T) {
	t.Parallel()

	store := newFakePlanStore()
	service := NewService(store, nil, nil)

	principal := reqctx.Principal{UserID: "user-1", Role: rbac.RoleIntern}

	_, err := service.UpdateItem(context.Background(), principal, "missing", UpdateItemInput{Status: "done"})
	require.ErrorIs(t, err, ErrItemNotFound)
}

func TestNextStepSkipsFinishedItems(t *testing.T) {
	t.Parallel()

	stages := []Stage{
		{Items: []Item{{ID: "a", Title: "Готово", Status: "done"}, {ID: "b", Title: "Пропущено", Status: "skipped"}}},
		{Items: []Item{{ID: "c", Title: "Следующий", Status: "pending"}}},
	}

	next := nextStepOf(stages)
	require.NotNil(t, next)
	require.Equal(t, "c", next.ItemID)
	require.Equal(t, "Следующий", next.Title)
	require.Contains(t, next.Reason, "Обязательный пункт")

	blocked := []Stage{
		{Items: []Item{{ID: "d", Title: "Курс", Status: "blocked", BlockedReason: "Ждём HR"}}},
	}

	next = nextStepOf(blocked)
	require.NotNil(t, next)
	require.Contains(t, next.Reason, "Ждём HR")

	require.Nil(t, nextStepOf([]Stage{{Items: []Item{{ID: "e", Status: "done"}}}}))
}

func TestProgressOf(t *testing.T) {
	t.Parallel()

	progress := progressOf([]Item{
		{Status: "done"},
		{Status: "done"},
		{Status: "pending"},
	})

	require.Equal(t, 2, progress.Done)
	require.Equal(t, 3, progress.Total)
	require.Equal(t, 66, progress.Percent)

	require.Equal(t, 0, progressOf(nil).Percent)
}

func TestCanEdit(t *testing.T) {
	t.Parallel()

	owner := reqctx.Principal{UserID: "user-1", Role: rbac.RoleIntern}
	require.True(t, canEdit(owner, "user-1"))
	require.False(t, canEdit(owner, "user-2"))

	mentor := reqctx.Principal{UserID: "mentor-1", Role: rbac.RoleMentor}
	require.True(t, canEdit(mentor, "user-2"))

	admin := reqctx.Principal{UserID: "admin-1", Role: rbac.RoleAdmin}
	require.True(t, canEdit(admin, "user-2"))

	other := reqctx.Principal{UserID: "user-3", Role: rbac.RoleIntern}
	require.False(t, canEdit(other, "user-2"))
}

func TestServicePlanRequiresStore(t *testing.T) {
	t.Parallel()

	service := NewService(nil, nil, nil)

	_, err := service.Plan(context.Background(), "user-1")
	require.Error(t, err)

	_, err = service.PlanForIntern(context.Background(), reqctx.Principal{}, "user-1")
	require.Error(t, err)
}

func TestUpdatedAtIsSet(t *testing.T) {
	t.Parallel()

	store := newFakePlanStore()
	store.items["item-2"] = Item{ID: "item-2", OwnerID: "user-1", Status: "pending"}

	service := NewService(store, nil, nil)

	principal := reqctx.Principal{UserID: "user-1", Role: rbac.RoleIntern}

	item, err := service.UpdateItem(context.Background(), principal, "item-2", UpdateItemInput{Status: "done"})
	require.NoError(t, err)
	require.Equal(t, "done", item.Status)
	require.NotNil(t, item.CompletedAt)
	require.WithinDuration(t, time.Now(), *item.CompletedAt, time.Minute)
}
