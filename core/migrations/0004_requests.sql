CREATE TABLE IF NOT EXISTS requests (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind            text NOT NULL DEFAULT 'absence'
                    CHECK (kind IN ('absence', 'access', 'handoff')),
    status          text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),
    title           text NOT NULL DEFAULT '',
    payload         jsonb NOT NULL DEFAULT '{}'::jsonb,
    external_id     text NOT NULL DEFAULT '',
    preview_id      text NOT NULL DEFAULT '',
    idempotency_key text NOT NULL DEFAULT '',
    approver_id     uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS requests_idempotency_uniq
    ON requests (user_id, kind, idempotency_key)
    WHERE idempotency_key <> '';

CREATE INDEX IF NOT EXISTS requests_user_idx ON requests (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS requests_status_idx ON requests (status);

CREATE TABLE IF NOT EXISTS request_events (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id  uuid NOT NULL REFERENCES requests (id) ON DELETE CASCADE,
    from_status text NOT NULL DEFAULT '',
    to_status   text NOT NULL DEFAULT '',
    comment     text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS request_events_request_idx ON request_events (request_id, created_at);
