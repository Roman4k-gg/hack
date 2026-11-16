package chat

import (
	"context"
	"fmt"
	"time"
)

type Conversation struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
}

type ToolCall struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type Citation struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
}

type HistoryMessage struct {
	ID        string     `json:"id"`
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls"`
	Citations []Citation `json:"citations"`
	CreatedAt time.Time  `json:"created_at"`
}

type HistoryStore interface {
	CreateConversation(ctx context.Context, userID, title string) (Conversation, error)
	Conversation(ctx context.Context, userID, conversationID string) (Conversation, error)
	ListConversations(ctx context.Context, userID string, limit int) ([]Conversation, error)
	AppendMessage(ctx context.Context, conversationID, role, content string, toolCalls []ToolCall, citations []Citation) error
	Messages(ctx context.Context, conversationID string) ([]HistoryMessage, error)
}

var ErrConversationNotFound = fmt.Errorf("chat: conversation not found")
var ErrConversationForbidden = fmt.Errorf("chat: conversation belongs to another user")
