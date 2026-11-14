package requests

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type RedisTokenStore struct {
	client *goredis.Client
}

func NewRedisTokenStore(client *goredis.Client) *RedisTokenStore {
	return &RedisTokenStore{client: client}
}

func (s *RedisTokenStore) PutConfirmation(ctx context.Context, token string, payload PreviewPayload, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("requests: redis client is not initialised")
	}

	raw, err := marshalPreview(payload)
	if err != nil {
		return err
	}

	if err := s.client.Set(ctx, confirmationKey(token), raw, ttl).Err(); err != nil {
		return fmt.Errorf("requests: store confirmation token: %w", err)
	}

	return nil
}

func (s *RedisTokenStore) TakeConfirmation(ctx context.Context, token string) (PreviewPayload, bool, error) {
	if s == nil || s.client == nil {
		return PreviewPayload{}, false, fmt.Errorf("requests: redis client is not initialised")
	}

	raw, err := s.client.GetDel(ctx, confirmationKey(token)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return PreviewPayload{}, false, nil
		}

		return PreviewPayload{}, false, fmt.Errorf("requests: take confirmation token: %w", err)
	}

	payload, err := unmarshalPreview(raw)
	if err != nil {
		return PreviewPayload{}, false, err
	}

	return payload, true, nil
}

func (s *RedisTokenStore) PutIdempotency(ctx context.Context, userID, kind, key, requestID string, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("requests: redis client is not initialised")
	}

	if err := s.client.Set(ctx, idempotencyKey(userID, kind, key), requestID, ttl).Err(); err != nil {
		return fmt.Errorf("requests: store idempotency key: %w", err)
	}

	return nil
}

func (s *RedisTokenStore) FindIdempotency(ctx context.Context, userID, kind, key string) (string, bool, error) {
	if s == nil || s.client == nil {
		return "", false, fmt.Errorf("requests: redis client is not initialised")
	}

	requestID, err := s.client.Get(ctx, idempotencyKey(userID, kind, key)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return "", false, nil
		}

		return "", false, fmt.Errorf("requests: find idempotency key: %w", err)
	}

	return requestID, true, nil
}

func confirmationKey(token string) string {
	return "requests:confirmation:" + token
}

func idempotencyKey(userID, kind, key string) string {
	return "requests:idempotency:" + userID + ":" + kind + ":" + key
}
