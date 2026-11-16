package auth

import (
	"context"
	"os"
	"testing"
	"time"

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

func TestPostgresUserStoreLoadsUserByEmail(t *testing.T) {
	store, ctx := testStore(t)
	userStore := NewPostgresUserStore(store.Pool())

	user, err := userStore.UserByEmail(ctx, "dasha@demo.local")
	require.NoError(t, err)
	require.Equal(t, demoInternID, user.ID)
	require.Equal(t, "Дарья Соколова", user.FullName)
	require.Equal(t, "intern", user.Role)
	require.Equal(t, "Search", user.TeamName)
	require.Equal(t, "Пётр Иванов", user.MentorName)
}

func TestPostgresUserStoreLoadsUserByID(t *testing.T) {
	store, ctx := testStore(t)
	userStore := NewPostgresUserStore(store.Pool())

	user, err := userStore.UserByID(ctx, demoInternID)
	require.NoError(t, err)
	require.Equal(t, "dasha@demo.local", user.Email)
	require.Equal(t, "Стажёр-разработчик", user.Position)
	require.Equal(t, "search", user.TeamSlug)
}

func TestPostgresUserStoreUnknownUser(t *testing.T) {
	store, ctx := testStore(t)
	userStore := NewPostgresUserStore(store.Pool())

	_, err := userStore.UserByEmail(ctx, "nobody@demo.local")
	require.ErrorIs(t, err, ErrUserNotFound)

	_, err = userStore.UserByID(ctx, "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, ErrUserNotFound)
}

func TestServiceLoginIssuesTokenWithScopes(t *testing.T) {
	store, ctx := testStore(t)

	tokens, err := NewTokenManager("secret", time.Hour, "ai-buddy-core")
	require.NoError(t, err)

	service := NewService(NewPostgresUserStore(store.Pool()), tokens)

	session, err := service.Login(ctx, "dasha@demo.local")
	require.NoError(t, err)
	require.NotEmpty(t, session.Token)
	require.Equal(t, "Bearer", session.TokenType)
	require.Contains(t, session.User.Scopes, "onboarding:plan:read")

	profile, err := service.Me(ctx, demoInternID, session.User.Scopes)
	require.NoError(t, err)
	require.Equal(t, "Дарья Соколова", profile.FullName)
	require.Equal(t, session.User.Scopes, profile.Scopes)
}

func TestServiceLoginValidatesInput(t *testing.T) {
	t.Parallel()

	tokens, err := NewTokenManager("secret", time.Hour, "ai-buddy-core")
	require.NoError(t, err)

	service := NewService(newFakeUserStore(), tokens)

	_, err = service.Login(context.Background(), "  ")
	require.ErrorIs(t, err, ErrEmailRequired)

	_, err = service.Login(context.Background(), "not-an-email")
	require.ErrorIs(t, err, ErrInvalidEmail)

	_, err = service.Me(context.Background(), "  ", nil)
	require.ErrorIs(t, err, ErrUserNotFound)
}
