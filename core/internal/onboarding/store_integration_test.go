package onboarding

import (
	"context"
	"os"
	"testing"

	"ai-buddy/core/internal/platform/db"
	"ai-buddy/core/internal/seed"

	"github.com/stretchr/testify/require"
)

const demoInternID = "22222222-2222-2222-2222-222222222224"

func testStore(t *testing.T) (*db.Store, context.Context) {
	t.Helper()

	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}

	ctx := context.Background()

	store, err := db.New(ctx, dsn, 2)
	require.NoError(t, err)

	t.Cleanup(func() { store.Close() })

	require.NoError(t, store.Migrate(ctx))
	require.NoError(t, seed.Run(ctx, store.Pool(), seed.Options{Demo: true}))

	return store, ctx
}

func TestPostgresPlanStoreLoadsPlan(t *testing.T) {
	store, ctx := testStore(t)
	planStore := NewPostgresPlanStore(store.Pool())

	plan, err := planStore.PlanByUserID(ctx, demoInternID)
	require.NoError(t, err)
	require.Equal(t, "Адаптация в команде Search", plan.Title)
	require.Equal(t, "Дарья Соколова", plan.Intern.FullName)
	require.Equal(t, "Search", plan.Intern.TeamName)
	require.Len(t, plan.Stages, 4)
	require.Len(t, plan.Stages[0].Items, 3)
	require.Equal(t, 40, plan.Progress.Percent)
	require.NotNil(t, plan.NextStep)
	require.Equal(t, "Получить доступ к Jira", plan.NextStep.Title)

	_, err = planStore.PlanByUserID(ctx, "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, ErrPlanNotFound)
}

func TestPostgresPlanStoreInternAccess(t *testing.T) {
	store, ctx := testStore(t)
	planStore := NewPostgresPlanStore(store.Pool())

	access, err := planStore.InternAccess(ctx, demoInternID)
	require.NoError(t, err)
	require.Equal(t, demoInternID, access.UserID)
	require.Equal(t, "11111111-1111-1111-1111-111111111111", access.TeamID)
	require.Equal(t, "22222222-2222-2222-2222-222222222221", access.MentorID)

	_, err = planStore.InternAccess(ctx, "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, ErrPlanNotFound)
}

func TestPostgresPlanStoreUpdatesItemStatus(t *testing.T) {
	store, ctx := testStore(t)
	planStore := NewPostgresPlanStore(store.Pool())

	item, err := planStore.ItemByID(ctx, "44444444-4444-4444-4444-444444444403")
	require.NoError(t, err)
	require.Equal(t, "in_progress", item.Status)
	require.Equal(t, demoInternID, item.OwnerID)

	updated, err := planStore.UpdateItemStatus(ctx, item.ID, "done", "")
	require.NoError(t, err)
	require.Equal(t, "done", updated.Status)
	require.NotNil(t, updated.CompletedAt)

	t.Cleanup(func() {
		_, _ = planStore.UpdateItemStatus(context.Background(), item.ID, "in_progress", "")
	})

	_, err = planStore.ItemByID(ctx, "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, ErrItemNotFound)

	_, err = planStore.UpdateItemStatus(ctx, "00000000-0000-0000-0000-000000000000", "done", "")
	require.ErrorIs(t, err, ErrItemNotFound)
}

func TestPostgresPlanStoreFailsWithoutPool(t *testing.T) {
	t.Parallel()

	planStore := NewPostgresPlanStore(nil)

	_, err := planStore.PlanByUserID(context.Background(), "x")
	require.Error(t, err)

	_, err = planStore.ItemByID(context.Background(), "x")
	require.Error(t, err)

	_, err = planStore.UpdateItemStatus(context.Background(), "x", "done", "")
	require.Error(t, err)

	_, err = planStore.InternAccess(context.Background(), "x")
	require.Error(t, err)
}
