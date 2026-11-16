package team

import (
	"context"
	"os"
	"testing"

	"ai-buddy/core/internal/platform/db"
	"ai-buddy/core/internal/seed"

	"github.com/stretchr/testify/require"
)

const (
	teamSearchID = "11111111-1111-1111-1111-111111111111"
	teamAdsID    = "11111111-1111-1111-1111-111111111112"
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

func TestPostgresStoreSnapshotForMentor(t *testing.T) {
	store, ctx := testStore(t)
	teamStore := NewPostgresStore(store.Pool())

	snapshot, err := teamStore.Snapshot(ctx, Filter{TeamID: teamSearchID, MentorID: "22222222-2222-2222-2222-222222222221"})
	require.NoError(t, err)
	require.NotEmpty(t, snapshot.Interns)
	require.NotEmpty(t, snapshot.Items)
	require.NotEmpty(t, snapshot.Activity)

	for _, intern := range snapshot.Interns {
		require.Equal(t, "Search", intern.TeamName)
		require.Equal(t, "Пётр Иванов", intern.MentorName)
	}
}

func TestPostgresStoreSnapshotForAdminSeesEveryone(t *testing.T) {
	store, ctx := testStore(t)
	teamStore := NewPostgresStore(store.Pool())

	snapshot, err := teamStore.Snapshot(ctx, Filter{All: true})
	require.NoError(t, err)
	require.Len(t, snapshot.Interns, 6)

	teams := map[string]bool{}
	for _, intern := range snapshot.Interns {
		teams[intern.TeamName] = true
	}

	require.True(t, teams["Search"])
	require.True(t, teams["Ads"])
	require.True(t, teams["Platform"])
}

func TestPostgresStoreSnapshotForForeignTeamIsEmpty(t *testing.T) {
	store, ctx := testStore(t)
	teamStore := NewPostgresStore(store.Pool())

	snapshot, err := teamStore.Snapshot(ctx, Filter{TeamID: "team-does-not-exist", MentorID: "mentor-does-not-exist"})
	require.NoError(t, err)
	require.Empty(t, snapshot.Interns)
	require.Empty(t, snapshot.Items)
}

func TestPostgresStoreFailsWithoutPool(t *testing.T) {
	t.Parallel()

	teamStore := NewPostgresStore(nil)

	_, err := teamStore.Snapshot(context.Background(), Filter{})
	require.Error(t, err)
}
