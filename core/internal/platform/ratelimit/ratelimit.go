package ratelimit

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Backend interface {
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

type RedisBackend struct {
	client *goredis.Client
}

func NewRedisBackend(client *goredis.Client) *RedisBackend {
	return &RedisBackend{client: client}
}

func (b *RedisBackend) Incr(ctx context.Context, key string) (int64, error) {
	if b == nil || b.client == nil {
		return 0, fmt.Errorf("ratelimit: redis client is not initialised")
	}
	count, err := b.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("ratelimit: incr %s: %w", key, err)
	}
	return count, nil
}

func (b *RedisBackend) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if b == nil || b.client == nil {
		return fmt.Errorf("ratelimit: redis client is not initialised")
	}
	if err := b.client.Expire(ctx, key, ttl).Err(); err != nil {
		return fmt.Errorf("ratelimit: expire %s: %w", key, err)
	}
	return nil
}

type Limiter struct {
	backend Backend
	limit   int
	window  time.Duration
}

func New(backend Backend, limit int) *Limiter {
	return &Limiter{
		backend: backend,
		limit:   limit,
		window:  time.Second,
	}
}

func (l *Limiter) Limit() int {
	return l.limit
}

func (l *Limiter) Allow(ctx context.Context, key string) (bool, error) {
	if l == nil || l.backend == nil {
		return false, fmt.Errorf("ratelimit: backend is not initialised")
	}
	if l.limit <= 0 {
		return true, nil
	}

	count, err := l.backend.Incr(ctx, key)
	if err != nil {
		return false, err
	}
	if count == 1 {
		if err := l.backend.Expire(ctx, key, l.window); err != nil {
			return false, err
		}
	}

	return count <= int64(l.limit), nil
}
