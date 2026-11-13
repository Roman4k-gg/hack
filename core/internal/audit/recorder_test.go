package audit

import (
	"context"
	"os"
	"testing"

	"ai-buddy/core/internal/platform/db"
	"ai-buddy/core/internal/platform/logging"

	"github.com/stretchr/testify/require"
)

func TestRecorderWritesAuditLogAndEvents(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}

	ctx := context.Background()
	store, err := db.New(ctx, dsn, 2)
	require.NoError(t, err)
	defer store.Close()

	require.NoError(t, store.Migrate(ctx))

	logger := logging.New("error", "audit-test")
	recorder := New(store.Pool(), logger)

	actorID := "22222222-2222-2222-2222-222222222224"

	require.NoError(t, recorder.Record(ctx, Entry{
		ActorID:    actorID,
		Action:     "onboarding.item_updated",
		EntityType: "onboarding_item",
		EntityID:   "44444444-4444-4444-4444-444444444403",
		Payload:    map[string]any{"from": "pending", "to": "done"},
		TraceID:    "trace-audit",
	}))

	require.NoError(t, recorder.Emit(ctx, Event{
		UserID:     actorID,
		Name:       "onboarding_item_updated",
		EntityType: "onboarding_item",
		EntityID:   "44444444-4444-4444-4444-444444444403",
		Payload:    map[string]any{"status": "done"},
		TraceID:    "trace-audit",
	}))

	var auditRows int
	require.NoError(t, store.Pool().QueryRow(ctx,
		`SELECT count(*) FROM audit_log WHERE action = 'onboarding.item_updated' AND trace_id = 'trace-audit'`).Scan(&auditRows))
	require.GreaterOrEqual(t, auditRows, 1)

	var eventRows int
	require.NoError(t, store.Pool().QueryRow(ctx,
		`SELECT count(*) FROM events WHERE name = 'onboarding_item_updated' AND trace_id = 'trace-audit'`).Scan(&eventRows))
	require.GreaterOrEqual(t, eventRows, 1)
}

func TestRecorderWithoutPoolFails(t *testing.T) {
	t.Parallel()

	recorder := New(nil, nil)

	err := recorder.Record(context.Background(), Entry{Action: "test"})
	require.Error(t, err)

	err = recorder.Emit(context.Background(), Event{Name: "test"})
	require.Error(t, err)
}
