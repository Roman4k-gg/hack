package flags

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) List(ctx context.Context) ([]Flag, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("flags: postgres pool is not initialised")
	}

	rows, err := s.pool.Query(ctx, `
		SELECT key, enabled, rollout, description, updated_at
		  FROM feature_flags
		 ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("flags: load flags: %w", err)
	}
	defer rows.Close()

	var flags []Flag

	for rows.Next() {
		var flag Flag
		if err := rows.Scan(&flag.Key, &flag.Enabled, &flag.Rollout, &flag.Description, &flag.UpdatedAt); err != nil {
			return nil, fmt.Errorf("flags: scan flag: %w", err)
		}

		flags = append(flags, flag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("flags: iterate flags: %w", err)
	}

	return flags, nil
}

func (s *PostgresStore) Set(ctx context.Context, key string, enabled bool, rollout int, updatedBy string) (Flag, error) {
	if s == nil || s.pool == nil {
		return Flag{}, fmt.Errorf("flags: postgres pool is not initialised")
	}

	var updatedByValue any
	if updatedBy != "" {
		updatedByValue = updatedBy
	}

	var flag Flag

	err := s.pool.QueryRow(ctx, `
		INSERT INTO feature_flags (key, enabled, rollout, updated_by, updated_at)
		VALUES ($1, $2, $3, $4::uuid, now())
		ON CONFLICT (key) DO UPDATE
		SET enabled = EXCLUDED.enabled,
		    rollout = EXCLUDED.rollout,
		    updated_by = EXCLUDED.updated_by,
		    updated_at = now()
		RETURNING key, enabled, rollout, description, updated_at`,
		key, enabled, rollout, updatedByValue,
	).Scan(&flag.Key, &flag.Enabled, &flag.Rollout, &flag.Description, &flag.UpdatedAt)
	if err != nil {
		return Flag{}, fmt.Errorf("flags: save flag: %w", err)
	}

	return flag, nil
}

func (s *PostgresStore) Get(ctx context.Context, key string) (Flag, error) {
	if s == nil || s.pool == nil {
		return Flag{}, fmt.Errorf("flags: postgres pool is not initialised")
	}

	var flag Flag

	err := s.pool.QueryRow(ctx, `
		SELECT key, enabled, rollout, description, updated_at
		  FROM feature_flags
		 WHERE key = $1`,
		key,
	).Scan(&flag.Key, &flag.Enabled, &flag.Rollout, &flag.Description, &flag.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Flag{}, ErrFlagNotFound
		}

		return Flag{}, fmt.Errorf("flags: load flag: %w", err)
	}

	return flag, nil
}
