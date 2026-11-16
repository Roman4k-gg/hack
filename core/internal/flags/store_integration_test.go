package flags

import (
	"context"
	"os"
	"testing"

	"ai-buddy/core/internal/platform/db"

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

	return store, ctx
}

func TestPostgresStoreListsSeededFlags(t *testing.T) {
	store, ctx := testStore(t)
	flagsStore := NewPostgresStore(store.Pool())

	flags, err := flagsStore.List(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, flags)

	keys := map[string]bool{}
	for _, flag := range flags {
		keys[flag.Key] = true
	}

	require.True(t, keys["kb_citations"])
	require.True(t, keys["mentor_dashboard"])
}

func TestPostgresStoreSetsAndReadsFlag(t *testing.T) {
	store, ctx := testStore(t)
	flagsStore := NewPostgresStore(store.Pool())

	updated, err := flagsStore.Set(ctx, "kb_citations", false, 50, "")
	require.NoError(t, err)
	require.Equal(t, "kb_citations", updated.Key)
	require.False(t, updated.Enabled)
	require.Equal(t, 50, updated.Rollout)

	t.Cleanup(func() {
		_, _ = flagsStore.Set(context.Background(), "kb_citations", true, 100, "")
	})

	flag, err := flagsStore.Get(ctx, "kb_citations")
	require.NoError(t, err)
	require.False(t, flag.Enabled)
	require.Equal(t, 50, flag.Rollout)
}

func TestPostgresStoreCreatesNewFlag(t *testing.T) {
	store, ctx := testStore(t)
	flagsStore := NewPostgresStore(store.Pool())

	created, err := flagsStore.Set(ctx, "integration_test_flag", true, 100, "")
	require.NoError(t, err)
	require.Equal(t, "integration_test_flag", created.Key)

	t.Cleanup(func() {
		_, _ = store.Pool().Exec(context.Background(), `DELETE FROM feature_flags WHERE key = 'integration_test_flag'`)
	})
}

func TestPostgresStoreMissingFlag(t *testing.T) {
	store, ctx := testStore(t)
	flagsStore := NewPostgresStore(store.Pool())

	_, err := flagsStore.Get(ctx, "unknown_flag")
	require.ErrorIs(t, err, ErrFlagNotFound)
}

func TestPostgresStoreFailsWithoutPool(t *testing.T) {
	t.Parallel()

	flagsStore := NewPostgresStore(nil)

	_, err := flagsStore.List(context.Background())
	require.Error(t, err)

	_, err = flagsStore.Set(context.Background(), "key", true, 100, "")
	require.Error(t, err)

	_, err = flagsStore.Get(context.Background(), "key")
	require.Error(t, err)
}
