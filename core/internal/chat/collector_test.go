package chat

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCollectorGathersText(t *testing.T) {
	t.Parallel()

	collector := NewCollector()

	collector.Feed([]byte("event: message\ndata: {\"type\":\"delta\",\"text\":\"Спроси \"}\n\n"))
	collector.Feed([]byte("event: message\ndata: {\"type\":\"delta\",\"text\":\"наставника\"}\n\n"))

	content, toolCalls, citations := collector.Result()

	require.Equal(t, "Спроси наставника", content)
	require.Empty(t, toolCalls)
	require.Empty(t, citations)
}

func TestCollectorGathersToolCalls(t *testing.T) {
	t.Parallel()

	collector := NewCollector()

	collector.Feed([]byte("event: message\ndata: {\"type\":\"tool_call\",\"toolCall\":{\"id\":\"call-1\",\"name\":\"get_onboarding_plan\",\"status\":\"running\"}}\n\n"))
	collector.Feed([]byte("event: message\ndata: {\"type\":\"tool_call\",\"toolCall\":{\"id\":\"call-1\",\"name\":\"get_onboarding_plan\",\"status\":\"success\"}}\n\n"))

	_, toolCalls, _ := collector.Result()

	require.Len(t, toolCalls, 1)
	require.Equal(t, "get_onboarding_plan", toolCalls[0].Name)
	require.Equal(t, "success", toolCalls[0].Status)
}

func TestCollectorGathersCitations(t *testing.T) {
	t.Parallel()

	collector := NewCollector()

	collector.Feed([]byte("event: message\ndata: {\"type\":\"citation\",\"citation\":{\"id\":\"doc-1\",\"title\":\"Доступ к Jira\",\"url\":\"https://demo.local/docs/jira-access\"}}\n\n"))

	_, _, citations := collector.Result()

	require.Len(t, citations, 1)
	require.Equal(t, "Доступ к Jira", citations[0].Title)
	require.Equal(t, "https://demo.local/docs/jira-access", citations[0].URL)
}

func TestCollectorIgnoresBrokenEvents(t *testing.T) {
	t.Parallel()

	collector := NewCollector()

	collector.Feed([]byte(": heartbeat\n\n"))
	collector.Feed([]byte("event: message\ndata: {не json}\n\n"))
	collector.Feed([]byte("event: message\ndata: {\"type\":\"delta\",\"text\":\"ок\"}\n\n"))

	content, _, _ := collector.Result()

	require.Equal(t, "ок", content)
}

func TestCollectorHandlesMultilineData(t *testing.T) {
	t.Parallel()

	collector := NewCollector()

	collector.Feed([]byte("event: message\ndata: {\"type\":\"delta\",\ndata: \"text\":\"часть\"}\n\n"))

	content, _, _ := collector.Result()

	require.Equal(t, "часть", content)
}

func TestConversationTitle(t *testing.T) {
	t.Parallel()

	require.Equal(t, "Где взять доступ к Jira?", conversationTitle("  Где взять доступ к Jira?  "))
	require.Len(t, conversationTitle(string(make([]byte, 200))), 60)
}
