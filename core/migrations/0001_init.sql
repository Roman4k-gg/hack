CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TABLE IF NOT EXISTS teams (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug       text NOT NULL UNIQUE,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email      text NOT NULL,
    full_name  text NOT NULL,
    role       text NOT NULL CHECK (role IN ('intern', 'mentor', 'hr', 'admin')),
    team_id    uuid REFERENCES teams (id) ON DELETE SET NULL,
    mentor_id  uuid REFERENCES users (id) ON DELETE SET NULL,
    position   text NOT NULL DEFAULT '',
    avatar_url text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS users_email_uniq ON users (lower(email));
CREATE INDEX IF NOT EXISTS users_team_idx ON users (team_id);
CREATE INDEX IF NOT EXISTS users_mentor_idx ON users (mentor_id);

CREATE TABLE IF NOT EXISTS onboarding_plans (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title      text NOT NULL,
    starts_on  date NOT NULL DEFAULT current_date,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS onboarding_plans_user_uniq ON onboarding_plans (user_id);

CREATE TABLE IF NOT EXISTS onboarding_stages (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id     uuid NOT NULL REFERENCES onboarding_plans (id) ON DELETE CASCADE,
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    position    integer NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (plan_id, position)
);

CREATE TABLE IF NOT EXISTS onboarding_items (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id        uuid NOT NULL REFERENCES onboarding_plans (id) ON DELETE CASCADE,
    stage_id       uuid NOT NULL REFERENCES onboarding_stages (id) ON DELETE CASCADE,
    title          text NOT NULL,
    description    text NOT NULL DEFAULT '',
    kind           text NOT NULL DEFAULT 'task'
                   CHECK (kind IN ('task', 'doc', 'meeting', 'access')),
    status         text NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'in_progress', 'done', 'blocked', 'skipped')),
    blocked_reason text,
    due_on         date,
    position       integer NOT NULL,
    completed_at   timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (stage_id, position)
);

CREATE INDEX IF NOT EXISTS onboarding_items_plan_idx ON onboarding_items (plan_id, position);
CREATE INDEX IF NOT EXISTS onboarding_items_status_idx ON onboarding_items (plan_id, status);

CREATE TABLE IF NOT EXISTS kb_documents (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug       text NOT NULL UNIQUE,
    title      text NOT NULL,
    summary    text NOT NULL DEFAULT '',
    content    text NOT NULL DEFAULT '',
    url        text NOT NULL DEFAULT '',
    team_id    uuid REFERENCES teams (id) ON DELETE SET NULL,
    tags       text[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS kb_documents_tags_idx ON kb_documents USING gin (tags);
CREATE INDEX IF NOT EXISTS kb_documents_team_idx ON kb_documents (team_id);

CREATE TABLE IF NOT EXISTS audit_log (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id    uuid REFERENCES users (id) ON DELETE SET NULL,
    action      text NOT NULL,
    entity_type text NOT NULL,
    entity_id   text NOT NULL DEFAULT '',
    payload     jsonb NOT NULL DEFAULT '{}'::jsonb,
    trace_id    text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS audit_log_created_idx ON audit_log (created_at DESC);
CREATE INDEX IF NOT EXISTS audit_log_actor_idx ON audit_log (actor_id);
CREATE INDEX IF NOT EXISTS audit_log_entity_idx ON audit_log (entity_type, entity_id);

CREATE TABLE IF NOT EXISTS events (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid REFERENCES users (id) ON DELETE SET NULL,
    name        text NOT NULL,
    entity_type text NOT NULL DEFAULT '',
    entity_id   text NOT NULL DEFAULT '',
    payload     jsonb NOT NULL DEFAULT '{}'::jsonb,
    trace_id    text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS events_created_idx ON events (created_at DESC);
CREATE INDEX IF NOT EXISTS events_user_idx ON events (user_id);
CREATE INDEX IF NOT EXISTS events_name_idx ON events (name);

DO $$
DECLARE
    table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'teams',
        'users',
        'onboarding_plans',
        'onboarding_stages',
        'onboarding_items',
        'kb_documents'
    ]
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS %I ON %I', table_name || '_set_updated_at', table_name);
        EXECUTE format(
            'CREATE TRIGGER %I BEFORE UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION set_updated_at()',
            table_name || '_set_updated_at',
            table_name
        );
    END LOOP;
END;
$$;
