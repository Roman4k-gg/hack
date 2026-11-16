package requests

import (
	"context"
	"os"
	"testing"

	"ai-buddy/core/internal/platform/db"
	"ai-buddy/core/internal/seed"

	"github.com/stretchr/testify/require"
)

const demoInternID = "22222222-2222-2222-2222-222222222224"

func testStore(t *testing.T) (*db.Store, context.Context) {
	t.Helper()

	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}

	ctx := context.Background()

	store, err := db.New(ctx, dsn, 2)
	require.NoError(t, err)

	t.Cleanup(func() { store.Close() })

	require.NoError(t, store.Migrate(ctx))
	require.NoError(t, seed.Run(ctx, store.Pool(), seed.Options{Demo: true}))

	return store, ctx
}

func TestPostgresStoreCreatesAndLoadsRequest(t *testing.T) {
	store, ctx := testStore(t)
	requestsStore := NewPostgresStore(store.Pool())

	created, err := requestsStore.Create(ctx, Record{
		UserID:         demoInternID,
		Kind:           KindAbsence,
		Status:         StatusPending,
		Title:          "Отгул 20.11.2026",
		Payload:        AbsenceDraft{DateFrom: "2026-11-20", DateTo: "2026-11-20", AbsenceType: "day_off"},
		ExternalID:     "ABS-TEST",
		PreviewID:      "preview-test",
		IdempotencyKey: "store-test-1",
		Position:       1,
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.NotNil(t, created.Approver)
	require.Equal(t, "Ольга Кузнецова", created.Approver.FullName)

	t.Cleanup(func() {
		_, _ = store.Pool().Exec(context.Background(), `DELETE FROM requests WHERE idempotency_key = 'store-test-1'`)
	})

	loaded, err := requestsStore.ByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.Title, loaded.Title)
	require.Equal(t, "day_off", loaded.Payload.AbsenceType)
	require.Equal(t, 1, loaded.Position)

	found, err := requestsStore.ByIdempotencyKey(ctx, demoInternID, KindAbsence, "store-test-1")
	require.NoError(t, err)
	require.Equal(t, created.ID, found.ID)

	missing, err := requestsStore.ByIdempotencyKey(ctx, demoInternID, KindAbsence, "store-test-unknown")
	require.NoError(t, err)
	require.Empty(t, missing.ID)

	_, err = requestsStore.ByID(ctx, "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, ErrRequestNotFound)
}

func TestPostgresStoreListsRequestsAndEvents(t *testing.T) {
	store, ctx := testStore(t)
	requestsStore := NewPostgresStore(store.Pool())

	created, err := requestsStore.Create(ctx, Record{
		UserID:         demoInternID,
		Kind:           KindAbsence,
		Status:         StatusPending,
		Title:          "Отгул для проверки списка",
		Payload:        AbsenceDraft{DateFrom: "2026-11-21", AbsenceType: "day_off"},
		ExternalID:     "ABS-LIST",
		IdempotencyKey: "store-test-list",
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = store.Pool().Exec(context.Background(), `DELETE FROM requests WHERE idempotency_key = 'store-test-list'`)
	})

	require.NoError(t, requestsStore.AppendEvent(ctx, created.ID, "", StatusPending, "Создана стажёром"))

	records, err := requestsStore.ListByUser(ctx, demoInternID)
	require.NoError(t, err)
	require.NotEmpty(t, records)

	found := false
	for _, record := range records {
		if record.ID == created.ID {
			found = true
			require.Equal(t, "Отгул для проверки списка", record.Title)
		}
	}

	require.True(t, found, "созданная заявка должна попасть в список")

	events, err := requestsStore.Events(ctx, created.ID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, StatusPending, events[0].To)
	require.Equal(t, "Создана стажёром", events[0].Comment)
}

func TestPostgresStoreLoadsAccessBundle(t *testing.T) {
	store, ctx := testStore(t)
	requestsStore := NewPostgresStore(store.Pool())

	items, err := requestsStore.Bundle(ctx, "intern")
	require.NoError(t, err)
	require.Len(t, items, 4)
	require.Equal(t, "VPN", items[0].System)
	require.Equal(t, "Jira", items[1].System)
	require.NotEmpty(t, items[0].Reason)

	empty, err := requestsStore.Bundle(ctx, "unknown-bundle")
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestPostgresStoreFailsWithoutPool(t *testing.T) {
	t.Parallel()

	requestsStore := NewPostgresStore(nil)

	_, err := requestsStore.Create(context.Background(), Record{})
	require.Error(t, err)

	_, err = requestsStore.Bundle(context.Background(), "intern")
	require.Error(t, err)

	_, err = requestsStore.ByID(context.Background(), "x")
	require.Error(t, err)

	_, err = requestsStore.ListByUser(context.Background(), "x")
	require.Error(t, err)

	_, err = requestsStore.Events(context.Background(), "x")
	require.Error(t, err)

	require.Error(t, requestsStore.AppendEvent(context.Background(), "x", "", "pending", "c"))
}
