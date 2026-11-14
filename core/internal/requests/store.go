package requests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Create(ctx context.Context, record Record) (Record, error) {
	if s == nil || s.pool == nil {
		return Record{}, fmt.Errorf("requests: postgres pool is not initialised")
	}

	payload, err := json.Marshal(record.Payload)
	if err != nil {
		return Record{}, fmt.Errorf("requests: marshal payload: %w", err)
	}

	approverID, approverName, err := s.approver(ctx, record.UserID)
	if err != nil {
		return Record{}, err
	}

	var approverValue any
	if approverID != "" {
		approverValue = approverID
	}

	var createdID string

	err = s.pool.QueryRow(ctx, `
		INSERT INTO requests (user_id, kind, status, title, payload, external_id, preview_id, idempotency_key, approver_id, position)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, $9, $10)
		RETURNING id::text, created_at, updated_at`,
		record.UserID, record.Kind, record.Status, record.Title, string(payload),
		record.ExternalID, record.PreviewID, record.IdempotencyKey, approverValue, record.Position,
	).Scan(&createdID, &record.CreatedAt, &record.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			existing, findErr := s.ByIdempotencyKey(ctx, record.UserID, record.Kind, record.IdempotencyKey)
			if findErr == nil && existing.ID != "" {
				return existing, nil
			}
		}

		return Record{}, fmt.Errorf("requests: create request: %w", err)
	}

	record.ID = createdID

	if approverID != "" {
		record.Approver = &Person{ID: approverID, FullName: approverName}
	}

	return record, nil
}

func (s *PostgresStore) ByID(ctx context.Context, requestID string) (Record, error) {
	if s == nil || s.pool == nil {
		return Record{}, fmt.Errorf("requests: postgres pool is not initialised")
	}

	var record Record
	var payload []byte
	var approverID, approverName string

	err := s.pool.QueryRow(ctx, `
		SELECT r.id::text, r.user_id::text, r.kind, r.status, r.title, r.payload, r.external_id,
		       r.preview_id, r.idempotency_key, r.position, r.created_at, r.updated_at,
		       COALESCE(a.id::text, ''), COALESCE(a.full_name, '')
		  FROM requests r
		  LEFT JOIN users a ON a.id = r.approver_id
		 WHERE r.id::text = $1`,
		requestID,
	).Scan(
		&record.ID,
		&record.UserID,
		&record.Kind,
		&record.Status,
		&record.Title,
		&payload,
		&record.ExternalID,
		&record.PreviewID,
		&record.IdempotencyKey,
		&record.Position,
		&record.CreatedAt,
		&record.UpdatedAt,
		&approverID,
		&approverName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Record{}, ErrRequestNotFound
		}

		return Record{}, fmt.Errorf("requests: load request: %w", err)
	}

	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &record.Payload)
	}

	if approverID != "" {
		record.Approver = &Person{ID: approverID, FullName: approverName}
	}

	return record, nil
}

func (s *PostgresStore) ByIdempotencyKey(ctx context.Context, userID, kind, key string) (Record, error) {
	if s == nil || s.pool == nil {
		return Record{}, fmt.Errorf("requests: postgres pool is not initialised")
	}

	if key == "" {
		return Record{}, nil
	}

	var requestID string

	err := s.pool.QueryRow(ctx, `
		SELECT id::text
		  FROM requests
		 WHERE user_id::text = $1 AND kind = $2 AND idempotency_key = $3`,
		userID, kind, key,
	).Scan(&requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Record{}, nil
		}

		return Record{}, fmt.Errorf("requests: find by idempotency key: %w", err)
	}

	return s.ByID(ctx, requestID)
}

func (s *PostgresStore) ListByUser(ctx context.Context, userID string) ([]Record, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("requests: postgres pool is not initialised")
	}

	rows, err := s.pool.Query(ctx, `
		SELECT r.id::text, r.user_id::text, r.kind, r.status, r.title, r.payload, r.external_id,
		       r.preview_id, r.idempotency_key, r.position, r.created_at, r.updated_at,
		       COALESCE(a.id::text, ''), COALESCE(a.full_name, '')
		  FROM requests r
		  LEFT JOIN users a ON a.id = r.approver_id
		 WHERE r.user_id::text = $1
		 ORDER BY r.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("requests: list requests: %w", err)
	}
	defer rows.Close()

	var records []Record

	for rows.Next() {
		var record Record
		var payload []byte
		var approverID, approverName string

		if err := rows.Scan(
			&record.ID,
			&record.UserID,
			&record.Kind,
			&record.Status,
			&record.Title,
			&payload,
			&record.ExternalID,
			&record.PreviewID,
			&record.IdempotencyKey,
			&record.Position,
			&record.CreatedAt,
			&record.UpdatedAt,
			&approverID,
			&approverName,
		); err != nil {
			return nil, fmt.Errorf("requests: scan request: %w", err)
		}

		if len(payload) > 0 {
			_ = json.Unmarshal(payload, &record.Payload)
		}

		if approverID != "" {
			record.Approver = &Person{ID: approverID, FullName: approverName}
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("requests: iterate requests: %w", err)
	}

	return records, nil
}

func (s *PostgresStore) Events(ctx context.Context, requestID string) ([]Event, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("requests: postgres pool is not initialised")
	}

	rows, err := s.pool.Query(ctx, `
		SELECT from_status, to_status, comment, created_at
		  FROM request_events
		 WHERE request_id::text = $1
		 ORDER BY created_at`,
		requestID,
	)
	if err != nil {
		return nil, fmt.Errorf("requests: load events: %w", err)
	}
	defer rows.Close()

	events := []Event{}

	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.From, &event.To, &event.Comment, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("requests: scan event: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("requests: iterate events: %w", err)
	}

	return events, nil
}

func (s *PostgresStore) AppendEvent(ctx context.Context, requestID, from, to, comment string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("requests: postgres pool is not initialised")
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO request_events (request_id, from_status, to_status, comment)
		VALUES ($1::uuid, $2, $3, $4)`,
		requestID, from, to, comment,
	)
	if err != nil {
		return fmt.Errorf("requests: append event: %w", err)
	}

	return nil
}

func (s *PostgresStore) approver(ctx context.Context, userID string) (string, string, error) {
	var approverID, approverName string

	err := s.pool.QueryRow(ctx, `
		SELECT id::text, full_name
		  FROM users
		 WHERE team_id = (SELECT team_id FROM users WHERE id::text = $1)
		   AND role IN ('manager', 'mentor')
		 ORDER BY CASE role WHEN 'manager' THEN 0 ELSE 1 END, full_name
		 LIMIT 1`,
		userID,
	).Scan(&approverID, &approverName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", nil
		}

		return "", "", fmt.Errorf("requests: find approver: %w", err)
	}

	return approverID, approverName, nil
}

func (s *PostgresStore) Bundle(ctx context.Context, bundle string) ([]AccessItem, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("requests: postgres pool is not initialised")
	}

	rows, err := s.pool.Query(ctx, `
		SELECT system, reason, approver
		  FROM access_bundles
		 WHERE bundle = $1
		 ORDER BY position`,
		bundle,
	)
	if err != nil {
		return nil, fmt.Errorf("requests: load access bundle: %w", err)
	}
	defer rows.Close()

	items := []AccessItem{}

	for rows.Next() {
		var item AccessItem
		if err := rows.Scan(&item.System, &item.Reason, &item.Approver); err != nil {
			return nil, fmt.Errorf("requests: scan access item: %w", err)
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("requests: iterate access bundle: %w", err)
	}

	return items, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}

	return false
}
