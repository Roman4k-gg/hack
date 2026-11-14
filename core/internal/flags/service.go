package flags

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Flag struct {
	Key         string    `json:"key"`
	Enabled     bool      `json:"enabled"`
	Rollout     int       `json:"rollout"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Store interface {
	List(ctx context.Context) ([]Flag, error)
	Set(ctx context.Context, key string, enabled bool, rollout int, updatedBy string) (Flag, error)
	Get(ctx context.Context, key string) (Flag, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) List(ctx context.Context) ([]Flag, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("flags: store is not initialised")
	}

	flags, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}

	if flags == nil {
		flags = []Flag{}
	}

	return flags, nil
}

func (s *Service) Set(ctx context.Context, key string, enabled bool, rollout int, updatedBy string) (Flag, error) {
	if s == nil || s.store == nil {
		return Flag{}, fmt.Errorf("flags: store is not initialised")
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return Flag{}, ErrKeyRequired
	}

	if rollout < 0 || rollout > 100 {
		return Flag{}, ErrRolloutRange
	}

	return s.store.Set(ctx, key, enabled, rollout, updatedBy)
}

func (s *Service) Enabled(ctx context.Context, key string) bool {
	if s == nil || s.store == nil {
		return false
	}

	flag, err := s.store.Get(ctx, strings.TrimSpace(key))
	if err != nil {
		return false
	}

	return flag.Enabled && flag.Rollout > 0
}
