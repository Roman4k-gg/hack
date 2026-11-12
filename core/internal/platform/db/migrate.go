package db

import (
	"context"
	"fmt"
	"io/fs"
	"sort"

	"ai-buddy/core/migrations"

	"github.com/jackc/pgx/v5"
)

const migrationLockKey int64 = 0x61692d6275646479

const createSchemaMigrations = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    text PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
)`

func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("db: postgres pool is not initialised")
	}

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("db: acquire connection for migrations: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		return fmt.Errorf("db: acquire migration lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", migrationLockKey)
	}()

	if _, err := conn.Exec(ctx, createSchemaMigrations); err != nil {
		return fmt.Errorf("db: create schema_migrations: %w", err)
	}

	files, err := migrationFiles()
	if err != nil {
		return err
	}

	for _, name := range files {
		applied, err := migrationApplied(ctx, conn, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("db: read migration %s: %w", name, err)
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("db: begin migration %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("db: apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("db: record migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("db: commit migration %s: %w", name, err)
		}
	}

	return nil
}

func (s *Store) AppliedMigrations(ctx context.Context) ([]string, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("db: postgres pool is not initialised")
	}

	rows, err := s.pool.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("db: list applied migrations: %w", err)
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("db: scan migration version: %w", err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: iterate migration versions: %w", err)
	}
	return versions, nil
}

func migrationFiles() ([]string, error) {
	files, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("db: list embedded migrations: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("db: no embedded migrations found")
	}
	sort.Strings(files)
	return files, nil
}

func migrationApplied(ctx context.Context, conn interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, name string) (bool, error) {
	var applied bool
	err := conn.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", name,
	).Scan(&applied)
	if err != nil {
		return false, fmt.Errorf("db: check migration %s: %w", name, err)
	}
	return applied, nil
}
