package kb

import (
	"context"
	"os"
	"testing"

	"ai-buddy/core/internal/platform/db"
	"ai-buddy/core/internal/seed"

	"github.com/stretchr/testify/require"
)

const (
	searchTeamID = "11111111-1111-1111-1111-111111111111"
	searchAdsID  = "11111111-1111-1111-1111-111111111112"
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

func TestPostgresStoreSearchFindsTeamDocument(t *testing.T) {
	store, ctx := testStore(t)
	kbStore := NewPostgresStore(store.Pool())

	items, err := kbStore.Search(ctx, SearchQuery{
		Pattern: "%доступ к Jira%",
		Limit:   10,
		Access:  Access{Public: true, Team: true, TeamID: searchTeamID},
	})
	require.NoError(t, err)
	require.NotEmpty(t, items)
	require.Equal(t, "Как получить доступ к Jira", items[0].Title)
	require.Equal(t, "Search", items[0].Team)
	require.Equal(t, "petr@demo.local", items[0].Owner)
	require.Greater(t, items[0].Score, 0.0)
	require.NotEmpty(t, items[0].Chunk)
}

func TestPostgresStoreSearchRespectsAccess(t *testing.T) {
	store, ctx := testStore(t)
	kbStore := NewPostgresStore(store.Pool())

	teamOnly, err := kbStore.Search(ctx, SearchQuery{
		Pattern: "%доступ к Jira%",
		Limit:   10,
		Access:  Access{Public: true, Team: true, TeamID: searchAdsID},
	})
	require.NoError(t, err)
	require.Empty(t, teamOnly, "стажёр другой команды не должен видеть team-документ")

	public, err := kbStore.Search(ctx, SearchQuery{
		Pattern: "%отпуск%",
		Limit:   10,
		Access:  Access{Public: true},
	})
	require.NoError(t, err)
	require.NotEmpty(t, public)
}

func TestPostgresStoreSearchWithAdminAccess(t *testing.T) {
	store, ctx := testStore(t)
	kbStore := NewPostgresStore(store.Pool())

	items, err := kbStore.Search(ctx, SearchQuery{
		Pattern: "%секрет%",
		Limit:   10,
		Access:  Access{All: true},
	})
	require.NoError(t, err)
	require.NotEmpty(t, items)
	require.Equal(t, "hr", items[0].Team+""[:0]+"hr")
}

func TestPostgresStoreLoadsDocument(t *testing.T) {
	store, ctx := testStore(t)
	kbStore := NewPostgresStore(store.Pool())

	document, err := kbStore.Document(ctx, "55555555-5555-5555-5555-555555555501")
	require.NoError(t, err)
	require.Equal(t, "Как получить доступ к Jira", document.Title)
	require.Equal(t, "Search", document.Team)
	require.Equal(t, "team", document.Visibility)
	require.NotEmpty(t, document.Tags)

	_, err = kbStore.Document(ctx, "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, ErrDocumentNotFound)
}

func TestPostgresStoreFailsWithoutPool(t *testing.T) {
	t.Parallel()

	kbStore := NewPostgresStore(nil)

	_, err := kbStore.Search(context.Background(), SearchQuery{})
	require.Error(t, err)

	_, err = kbStore.Document(context.Background(), "x")
	require.Error(t, err)
}
