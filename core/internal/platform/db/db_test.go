package db

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRequiresDSN(t *testing.T) {
	t.Parallel()

	_, err := New(context.Background(), "   ", 2)
	require.Error(t, err)
}

func TestNewWithInvalidDSN(t *testing.T) {
	t.Parallel()

	store, err := New(context.Background(), "postgres://buddy:buddy@127.0.0.1:59999/buddy?sslmode=disable", 2)
	require.NoError(t, err)
	require.NotNil(t, store)

	require.Error(t, store.Ping(context.Background()))

	store.Close()
}

func TestMigrateIsIdempotent(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}

	ctx := context.Background()

	store, err := New(ctx, dsn, 2)
	require.NoError(t, err)

	t.Cleanup(func() { store.Close() })

	require.NoError(t, store.Migrate(ctx))
	require.NoError(t, store.Migrate(ctx))

	applied, err := store.AppliedMigrations(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, applied)
	require.Contains(t, applied, "0001_init.sql")
	require.Contains(t, applied, "0005_flags_access.sql")
}

func TestPingAndPool(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}

	ctx := context.Background()

	store, err := New(ctx, dsn, 2)
	require.NoError(t, err)

	t.Cleanup(func() { store.Close() })

	require.NoError(t, store.Ping(ctx))
	require.NotNil(t, store.Pool())
}
