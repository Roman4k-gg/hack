package ratelimit

import (
	"context"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisBackendIncrementsAndExpires(t *testing.T) {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR is not set")
	}

	ctx := context.Background()

	client := goredis.NewClient(&goredis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })

	require.NoError(t, client.Ping(ctx).Err())

	backend := NewRedisBackend(client)
	key := "ratelimit-test:" + time.Now().Format("150405.000000")

	t.Cleanup(func() {
		_ = client.Del(context.Background(), key).Err()
	})

	count, err := backend.Incr(ctx, key)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	count, err = backend.Incr(ctx, key)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)

	require.NoError(t, backend.Expire(ctx, key, time.Minute))

	ttl, err := client.TTL(ctx, key).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, time.Duration(0))
}

func TestLimiterWithRedisBackend(t *testing.T) {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR is not set")
	}

	ctx := context.Background()

	client := goredis.NewClient(&goredis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })

	key := "ratelimit-test:" + time.Now().Format("150405.000000")

	t.Cleanup(func() {
		_ = client.Del(context.Background(), key).Err()
	})

	limiter := New(NewRedisBackend(client), 2)
	require.Equal(t, 2, limiter.Limit())

	allowed, err := limiter.Allow(ctx, key)
	require.NoError(t, err)
	require.True(t, allowed)

	allowed, err = limiter.Allow(ctx, key)
	require.NoError(t, err)
	require.True(t, allowed)

	allowed, err = limiter.Allow(ctx, key)
	require.NoError(t, err)
	require.False(t, allowed, "третий запрос в окне должен быть отклонён")
}
