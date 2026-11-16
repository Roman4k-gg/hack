package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresHistoryStore struct {
	pool *pgxpool.Pool
}

func NewPostgresHistoryStore(pool *pgxpool.Pool) *PostgresHistoryStore {
	return &PostgresHistoryStore{pool: pool}
}

func (s *PostgresHistoryStore) CreateConversation(ctx context.Context, userID, title string) (Conversation, error) {
	if s == nil || s.pool == nil {
		return Conversation{}, fmt.Errorf("chat: postgres pool is not initialised")
	}

	var conversation Conversation

	err := s.pool.QueryRow(ctx, `
		INSERT INTO conversations (user_id, title)
		VALUES ($1, $2)
		RETURNING id::text, title, created_at, updated_at`,
		userID, title,
	).Scan(&conversation.ID, &conversation.Title, &conversation.CreatedAt, &conversation.UpdatedAt)
	if err != nil {
		return Conversation{}, fmt.Errorf("chat: create conversation: %w", err)
	}

	return conversation, nil
}

func (s *PostgresHistoryStore) Conversation(ctx context.Context, userID, conversationID string) (Conversation, error) {
	if s == nil || s.pool == nil {
		return Conversation{}, fmt.Errorf("chat: postgres pool is not initialised")
	}

	var conversation Conversation

	err := s.pool.QueryRow(ctx, `
		SELECT c.id::text, c.title, c.created_at, c.updated_at,
		       (SELECT count(*)::int FROM messages m WHERE m.conversation_id = c.id)
		  FROM conversations c
		 WHERE c.id::text = $1`,
		conversationID,
	).Scan(
		&conversation.ID,
		&conversation.Title,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
		&conversation.MessageCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Conversation{}, ErrConversationNotFound
		}

		return Conversation{}, fmt.Errorf("chat: load conversation: %w", err)
	}

	owner, err := s.ownerOf(ctx, conversationID)
	if err != nil {
		return Conversation{}, err
	}

	if owner != userID {
		return Conversation{}, ErrConversationForbidden
	}

	return conversation, nil
}

func (s *PostgresHistoryStore) ListConversations(ctx context.Context, userID string, limit int) ([]Conversation, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("chat: postgres pool is not initialised")
	}

	if limit <= 0 || limit > 50 {
		limit = 20
	}

	rows, err := s.pool.Query(ctx, `
		SELECT c.id::text, c.title, c.created_at, c.updated_at,
		       (SELECT count(*)::int FROM messages m WHERE m.conversation_id = c.id)
		  FROM conversations c
		 WHERE c.user_id::text = $1
		 ORDER BY c.updated_at DESC
		 LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("chat: list conversations: %w", err)
	}
	defer rows.Close()

	conversations := []Conversation{}

	for rows.Next() {
		var conversation Conversation
		if err := rows.Scan(
			&conversation.ID,
			&conversation.Title,
			&conversation.CreatedAt,
			&conversation.UpdatedAt,
			&conversation.MessageCount,
		); err != nil {
			return nil, fmt.Errorf("chat: scan conversation: %w", err)
		}

		conversations = append(conversations, conversation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: iterate conversations: %w", err)
	}

	return conversations, nil
}

func (s *PostgresHistoryStore) AppendMessage(
	ctx context.Context,
	conversationID, role, content string,
	toolCalls []ToolCall,
	citations []Citation,
) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("chat: postgres pool is not initialised")
	}

	if toolCalls == nil {
		toolCalls = []ToolCall{}
	}

	if citations == nil {
		citations = []Citation{}
	}

	toolPayload, err := json.Marshal(toolCalls)
	if err != nil {
		return fmt.Errorf("chat: marshal tool calls: %w", err)
	}

	citationPayload, err := json.Marshal(citations)
	if err != nil {
		return fmt.Errorf("chat: marshal citations: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO messages (conversation_id, role, content, tool_calls, citations)
		VALUES ($1::uuid, $2, $3, $4::jsonb, $5::jsonb)`,
		conversationID, role, content, string(toolPayload), string(citationPayload),
	)
	if err != nil {
		return fmt.Errorf("chat: append message: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE conversations SET updated_at = now() WHERE id::text = $1`, conversationID)
	if err != nil {
		return fmt.Errorf("chat: touch conversation: %w", err)
	}

	return nil
}

func (s *PostgresHistoryStore) Messages(ctx context.Context, conversationID string) ([]HistoryMessage, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("chat: postgres pool is not initialised")
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id::text, role, content, tool_calls, citations, created_at
		  FROM messages
		 WHERE conversation_id::text = $1
		 ORDER BY created_at`,
		conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("chat: load messages: %w", err)
	}
	defer rows.Close()

	messages := []HistoryMessage{}

	for rows.Next() {
		var message HistoryMessage
		var toolPayload, citationPayload []byte

		if err := rows.Scan(
			&message.ID,
			&message.Role,
			&message.Content,
			&toolPayload,
			&citationPayload,
			&message.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("chat: scan message: %w", err)
		}

		message.ToolCalls = []ToolCall{}
		message.Citations = []Citation{}

		if len(toolPayload) > 0 {
			_ = json.Unmarshal(toolPayload, &message.ToolCalls)
		}

		if len(citationPayload) > 0 {
			_ = json.Unmarshal(citationPayload, &message.Citations)
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat: iterate messages: %w", err)
	}

	return messages, nil
}

func (s *PostgresHistoryStore) ownerOf(ctx context.Context, conversationID string) (string, error) {
	var owner string

	err := s.pool.QueryRow(ctx, `
		SELECT user_id::text FROM conversations WHERE id::text = $1`, conversationID).Scan(&owner)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrConversationNotFound
		}

		return "", fmt.Errorf("chat: load conversation owner: %w", err)
	}

	return owner, nil
}
