package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeBackend struct {
	count   int64
	expired bool
	err     error
}

func (b *fakeBackend) Incr(_ context.Context, _ string) (int64, error) {
	if b.err != nil {
		return 0, b.err
	}
	b.count++
	return b.count, nil
}

func (b *fakeBackend) Expire(_ context.Context, _ string, _ time.Duration) error {
	if b.err != nil {
		return b.err
	}
	b.expired = true
	return nil
}

func TestLimiterAllowsUpToLimit(t *testing.T) {
	t.Parallel()

	backend := &fakeBackend{}
	limiter := New(backend, 2)

	allowed, err := limiter.Allow(context.Background(), "key")
	require.NoError(t, err)
	require.True(t, allowed)
	require.True(t, backend.expired)

	allowed, err = limiter.Allow(context.Background(), "key")
	require.NoError(t, err)
	require.True(t, allowed)

	allowed, err = limiter.Allow(context.Background(), "key")
	require.NoError(t, err)
	require.False(t, allowed)
}

func TestLimiterWithZeroLimitAllowsEverything(t *testing.T) {
	t.Parallel()

	limiter := New(&fakeBackend{}, 0)

	allowed, err := limiter.Allow(context.Background(), "key")
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, 0, limiter.Limit())
}

func TestLimiterPropagatesBackendError(t *testing.T) {
	t.Parallel()

	limiter := New(&fakeBackend{err: errors.New("redis is down")}, 1)

	_, err := limiter.Allow(context.Background(), "key")
	require.Error(t, err)
}

func TestLimiterWithoutBackendFails(t *testing.T) {
	t.Parallel()

	limiter := New(nil, 1)

	_, err := limiter.Allow(context.Background(), "key")
	require.Error(t, err)
}
