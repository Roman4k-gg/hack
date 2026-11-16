package chat

import (
	"context"
	"os"
	"testing"

	"ai-buddy/core/internal/platform/db"
	"ai-buddy/core/internal/seed"

	"github.com/stretchr/testify/require"
)

const demoInternID = "22222222-2222-2222-2222-222222222224"

func testHistory(t *testing.T) (*PostgresHistoryStore, context.Context) {
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

	return NewPostgresHistoryStore(store.Pool()), ctx
}

func TestHistoryCreatesConversationAndMessages(t *testing.T) {
	history, ctx := testHistory(t)

	conversation, err := history.CreateConversation(ctx, demoInternID, "Где взять доступ к Jira?")
	require.NoError(t, err)
	require.NotEmpty(t, conversation.ID)
	require.Equal(t, "Где взять доступ к Jira?", conversation.Title)

	t.Cleanup(func() {
		_, _ = history.pool.Exec(context.Background(), `DELETE FROM conversations WHERE id::text = $1`, conversation.ID)
	})

	require.NoError(t, history.AppendMessage(ctx, conversation.ID, "user", "Где взять доступ к Jira?", nil, nil))
	require.NoError(t, history.AppendMessage(ctx, conversation.ID, "assistant", "Спроси наставника",
		[]ToolCall{{ID: "call-1", Name: "get_onboarding_plan", Status: "success"}},
		[]Citation{{ID: "doc-1", Title: "Доступ к Jira", URL: "https://demo.local/docs/jira-access"}},
	))

	messages, err := history.Messages(ctx, conversation.ID)
	require.NoError(t, err)
	require.Len(t, messages, 2)
	require.Equal(t, "user", messages[0].Role)
	require.Equal(t, "Спроси наставника", messages[1].Content)
	require.Len(t, messages[1].ToolCalls, 1)
	require.Equal(t, "get_onboarding_plan", messages[1].ToolCalls[0].Name)
	require.Len(t, messages[1].Citations, 1)
	require.Equal(t, "Доступ к Jira", messages[1].Citations[0].Title)

	loaded, err := history.Conversation(ctx, demoInternID, conversation.ID)
	require.NoError(t, err)
	require.Equal(t, 2, loaded.MessageCount)
}

func TestHistoryListsConversationsOfUser(t *testing.T) {
	history, ctx := testHistory(t)

	conversation, err := history.CreateConversation(ctx, demoInternID, "Диалог про отпуск")
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = history.pool.Exec(context.Background(), `DELETE FROM conversations WHERE id::text = $1`, conversation.ID)
	})

	require.NoError(t, history.AppendMessage(ctx, conversation.ID, "user", "Хочу отгул", nil, nil))

	conversations, err := history.ListConversations(ctx, demoInternID, 10)
	require.NoError(t, err)
	require.NotEmpty(t, conversations)

	found := false
	for _, item := range conversations {
		if item.ID == conversation.ID {
			found = true
			require.Equal(t, "Диалог про отпуск", item.Title)
			require.Equal(t, 1, item.MessageCount)
		}
	}

	require.True(t, found, "диалог должен быть в списке")
}

func TestHistoryProtectsForeignConversation(t *testing.T) {
	history, ctx := testHistory(t)

	conversation, err := history.CreateConversation(ctx, demoInternID, "Чужой диалог")
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = history.pool.Exec(context.Background(), `DELETE FROM conversations WHERE id::text = $1`, conversation.ID)
	})

	_, err = history.Conversation(ctx, "22222222-2222-2222-2222-222222222229", conversation.ID)
	require.ErrorIs(t, err, ErrConversationForbidden)

	_, err = history.Conversation(ctx, demoInternID, "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, ErrConversationNotFound)
}

func TestHistoryStoreFailsWithoutPool(t *testing.T) {
	t.Parallel()

	history := NewPostgresHistoryStore(nil)

	_, err := history.CreateConversation(context.Background(), "user-1", "t")
	require.Error(t, err)

	_, err = history.Conversation(context.Background(), "user-1", "id")
	require.Error(t, err)

	_, err = history.ListConversations(context.Background(), "user-1", 10)
	require.Error(t, err)

	require.Error(t, history.AppendMessage(context.Background(), "id", "user", "t", nil, nil))

	_, err = history.Messages(context.Background(), "id")
	require.Error(t, err)
}
