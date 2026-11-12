package seed_test

import (
	"context"
	"os"
	"testing"

	"ai-buddy/core/internal/platform/db"
	"ai-buddy/core/internal/seed"

	"github.com/stretchr/testify/require"
)

func TestRunIsIdempotent(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}

	ctx := context.Background()
	store, err := db.New(ctx, dsn, 2)
	require.NoError(t, err)
	defer store.Close()

	require.NoError(t, store.Migrate(ctx))

	counts := func() (int, int) {
		var users, items int
		require.NoError(t, store.Pool().QueryRow(ctx,
			`SELECT count(*) FROM users WHERE email LIKE '%@demo.local'`).Scan(&users))
		require.NoError(t, store.Pool().QueryRow(ctx,
			`SELECT count(*)
			   FROM onboarding_items i
			   JOIN onboarding_plans p ON p.id = i.plan_id
			   JOIN users u ON u.id = p.user_id
			  WHERE u.email = 'dasha@demo.local'`).Scan(&items))
		return users, items
	}

	require.NoError(t, seed.Run(ctx, store.Pool(), seed.Options{Demo: true}))
	usersAfterFirst, itemsAfterFirst := counts()
	require.GreaterOrEqual(t, usersAfterFirst, 4)
	require.GreaterOrEqual(t, itemsAfterFirst, 10)

	require.NoError(t, seed.Run(ctx, store.Pool(), seed.Options{Demo: true}))
	usersAfterSecond, itemsAfterSecond := counts()
	require.Equal(t, usersAfterFirst, usersAfterSecond)
	require.Equal(t, itemsAfterFirst, itemsAfterSecond)

	var nextStep string
	require.NoError(t, store.Pool().QueryRow(ctx,
		`SELECT i.title
		   FROM onboarding_items i
		   JOIN onboarding_stages s ON s.id = i.stage_id
		   JOIN onboarding_plans p ON p.id = i.plan_id
		   JOIN users u ON u.id = p.user_id
		  WHERE u.email = 'dasha@demo.local'
		    AND i.status <> 'done'
		  ORDER BY s.position, i.position
		  LIMIT 1`).Scan(&nextStep))
	require.Equal(t, "Получить доступ к Jira", nextStep)
}
