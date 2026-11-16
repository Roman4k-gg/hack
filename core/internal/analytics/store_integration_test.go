package analytics

import (
	"context"
	"os"
	"testing"

	"ai-buddy/core/internal/platform/db"
	"ai-buddy/core/internal/seed"

	"github.com/stretchr/testify/require"
)

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

func TestPostgresStoreTopQuestions(t *testing.T) {
	store, ctx := testStore(t)
	analyticsStore := NewPostgresStore(store.Pool())

	questions, err := analyticsStore.TopQuestions(ctx, 10)
	require.NoError(t, err)
	require.NotEmpty(t, questions)

	for _, question := range questions {
		require.NotEmpty(t, question.Question)
		require.Greater(t, question.Count, 0)
	}
}

func TestPostgresStoreContentGaps(t *testing.T) {
	store, ctx := testStore(t)
	analyticsStore := NewPostgresStore(store.Pool())

	gaps, err := analyticsStore.ContentGaps(ctx, 20)
	require.NoError(t, err)

	reasons := map[string]bool{}
	for _, gap := range gaps {
		require.NotEmpty(t, gap.Topic)
		require.Contains(t, []string{"missing", "outdated"}, gap.Reason)
		reasons[gap.Reason] = true
	}

	require.True(t, reasons["missing"], "ожидается дыра без документа")
}

func TestPostgresStoreOverview(t *testing.T) {
	store, ctx := testStore(t)
	analyticsStore := NewPostgresStore(store.Pool())

	overview, err := analyticsStore.Overview(ctx)
	require.NoError(t, err)
	require.Greater(t, overview.Interns, 0)
	require.Greater(t, overview.Questions, 0)
	require.NotEmpty(t, overview.TopQuestions)
}

func TestPostgresStoreDocumentMatch(t *testing.T) {
	store, ctx := testStore(t)
	analyticsStore := NewPostgresStore(store.Pool())

	title, updatedAt, found, err := analyticsStore.DocumentMatch(ctx, "где взять доступ к Jira?")
	require.NoError(t, err)
	require.True(t, found)
	require.NotEmpty(t, title)
	require.False(t, updatedAt.IsZero())

	_, _, found, err = analyticsStore.DocumentMatch(ctx, "zzz-несуществующая-тема-98765")
	require.NoError(t, err)
	require.False(t, found)

	_, _, found, err = analyticsStore.DocumentMatch(ctx, "   ")
	require.NoError(t, err)
	require.False(t, found)
}

func TestPostgresStoreFailsWithoutPool(t *testing.T) {
	t.Parallel()

	analyticsStore := NewPostgresStore(nil)

	_, err := analyticsStore.TopQuestions(context.Background(), 10)
	require.Error(t, err)

	_, err = analyticsStore.ContentGaps(context.Background(), 10)
	require.Error(t, err)

	_, err = analyticsStore.Overview(context.Background())
	require.Error(t, err)

	_, _, _, err = analyticsStore.DocumentMatch(context.Background(), "jira")
	require.Error(t, err)
}
