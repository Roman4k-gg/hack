package flags

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	flags map[string]Flag
}

func newFakeStore() *fakeStore {
	return &fakeStore{flags: map[string]Flag{
		"kb_citations": {
			Key:         "kb_citations",
			Enabled:     true,
			Rollout:     100,
			Description: "Цитаты в ответах",
			UpdatedAt:   time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC),
		},
	}}
}

func (s *fakeStore) List(_ context.Context) ([]Flag, error) {
	flags := []Flag{}
	for _, flag := range s.flags {
		flags = append(flags, flag)
	}

	return flags, nil
}

func (s *fakeStore) Set(_ context.Context, key string, enabled bool, rollout int, _ string) (Flag, error) {
	flag := s.flags[key]
	flag.Key = key
	flag.Enabled = enabled
	flag.Rollout = rollout

	if flag.Description == "" {
		flag.Description = "Флаг без описания"
	}

	s.flags[key] = flag

	return flag, nil
}

func (s *fakeStore) Get(_ context.Context, key string) (Flag, error) {
	flag, ok := s.flags[key]
	if !ok {
		return Flag{}, ErrFlagNotFound
	}

	return flag, nil
}

func TestListReturnsFlags(t *testing.T) {
	t.Parallel()

	service := NewService(newFakeStore())

	flags, err := service.List(context.Background())
	require.NoError(t, err)
	require.Len(t, flags, 1)
	require.Equal(t, "kb_citations", flags[0].Key)
}

func TestSetUpdatesFlag(t *testing.T) {
	t.Parallel()

	service := NewService(newFakeStore())

	flag, err := service.Set(context.Background(), "kb_citations", false, 50, "user-1")
	require.NoError(t, err)
	require.False(t, flag.Enabled)
	require.Equal(t, 50, flag.Rollout)
}

func TestSetValidatesInput(t *testing.T) {
	t.Parallel()

	service := NewService(newFakeStore())

	_, err := service.Set(context.Background(), "  ", true, 100, "user-1")
	require.ErrorIs(t, err, ErrKeyRequired)

	_, err = service.Set(context.Background(), "kb_citations", true, 200, "user-1")
	require.ErrorIs(t, err, ErrRolloutRange)
}

func TestEnabledReadsFlag(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	service := NewService(store)

	require.True(t, service.Enabled(context.Background(), "kb_citations"))
	require.False(t, service.Enabled(context.Background(), "unknown_flag"))

	_, err := service.Set(context.Background(), "kb_citations", false, 100, "user-1")
	require.NoError(t, err)
	require.False(t, service.Enabled(context.Background(), "kb_citations"))
}

func TestServiceWithoutStoreFails(t *testing.T) {
	t.Parallel()

	service := NewService(nil)

	_, err := service.List(context.Background())
	require.Error(t, err)
}
