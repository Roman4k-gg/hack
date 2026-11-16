package requests

import (
	"context"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisTokenStoreRoundTrip(t *testing.T) {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR is not set")
	}

	ctx := context.Background()
	client := goredis.NewClient(&goredis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.Ping(ctx).Err())

	store := NewRedisTokenStore(client)
	suffix := time.Now().Format("150405.000000")
	token := "token-" + suffix

	payload := PreviewPayload{PreviewID: "preview-" + suffix, UserID: "user-1"}
	require.NoError(t, store.PutConfirmation(ctx, token, payload, time.Minute))

	taken, ok, err := store.TakeConfirmation(ctx, token)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, payload.PreviewID, taken.PreviewID)

	_, ok, err = store.TakeConfirmation(ctx, token)
	require.NoError(t, err)
	require.False(t, ok)

	require.NoError(t, client.Set(ctx, confirmationKey("broken-"+suffix), "{", time.Minute).Err())
	_, _, err = store.TakeConfirmation(ctx, "broken-"+suffix)
	require.Error(t, err)

	_, ok, err = store.FindIdempotency(ctx, "user-1", "absence", suffix)
	require.NoError(t, err)
	require.False(t, ok)

	require.NoError(t, store.PutIdempotency(ctx, "user-1", "absence", suffix, "request-1", time.Minute))
	requestID, ok, err := store.FindIdempotency(ctx, "user-1", "absence", suffix)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "request-1", requestID)
}

func TestRedisTokenStoreWithoutClient(t *testing.T) {
	t.Parallel()

	var store *RedisTokenStore
	ctx := context.Background()

	require.Error(t, store.PutConfirmation(ctx, "t", PreviewPayload{}, time.Minute))
	_, _, err := store.TakeConfirmation(ctx, "t")
	require.Error(t, err)
	require.Error(t, store.PutIdempotency(ctx, "u", "k", "key", "r", time.Minute))
	_, _, err = store.FindIdempotency(ctx, "u", "k", "key")
	require.Error(t, err)
}
