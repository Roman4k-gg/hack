package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Entry struct {
	ActorID    string
	Action     string
	EntityType string
	EntityID   string
	Payload    map[string]any
	TraceID    string
}

type Event struct {
	UserID     string
	Name       string
	EntityType string
	EntityID   string
	Payload    map[string]any
	TraceID    string
}

type Recorder struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func New(pool *pgxpool.Pool, logger *slog.Logger) *Recorder {
	return &Recorder{pool: pool, logger: logger}
}

func (r *Recorder) Record(ctx context.Context, entry Entry) error {
	if r == nil || r.pool == nil {
		return fmt.Errorf("audit: recorder is not initialised")
	}

	payload, err := marshalPayload(entry.Payload)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO audit_log (actor_id, action, entity_type, entity_id, payload, trace_id)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6)`,
		nullIfEmpty(entry.ActorID), entry.Action, entry.EntityType, entry.EntityID, payload, entry.TraceID,
	)
	if err != nil {
		return fmt.Errorf("audit: insert audit log: %w", err)
	}
	return nil
}

func (r *Recorder) Emit(ctx context.Context, event Event) error {
	if r == nil || r.pool == nil {
		return fmt.Errorf("audit: recorder is not initialised")
	}

	payload, err := marshalPayload(event.Payload)
	if err != nil {
		return err
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO events (user_id, name, entity_type, entity_id, payload, trace_id)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6)`,
		nullIfEmpty(event.UserID), event.Name, event.EntityType, event.EntityID, payload, event.TraceID,
	)
	if err != nil {
		return fmt.Errorf("audit: insert event: %w", err)
	}
	return nil
}

func marshalPayload(payload map[string]any) (string, error) {
	if payload == nil {
		return "{}", nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("audit: marshal payload: %w", err)
	}
	return string(raw), nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
