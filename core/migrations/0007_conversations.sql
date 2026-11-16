CREATE TABLE IF NOT EXISTS conversations (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS conversations_user_idx ON conversations (user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS messages (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    role            text NOT NULL CHECK (role IN ('user', 'assistant', 'system', 'tool')),
    content         text NOT NULL DEFAULT '',
    tool_calls      jsonb NOT NULL DEFAULT '[]'::jsonb,
    citations       jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS messages_conversation_idx ON messages (conversation_id, created_at);
