ALTER TABLE requests ADD COLUMN IF NOT EXISTS position integer NOT NULL DEFAULT 0;

DROP INDEX IF EXISTS requests_idempotency_uniq;

CREATE UNIQUE INDEX IF NOT EXISTS requests_idempotency_uniq
    ON requests (user_id, kind, idempotency_key, title)
    WHERE idempotency_key <> '';

CREATE TABLE IF NOT EXISTS feature_flags (
    key         text PRIMARY KEY,
    enabled     boolean NOT NULL DEFAULT true,
    rollout     integer NOT NULL DEFAULT 100 CHECK (rollout BETWEEN 0 AND 100),
    description text NOT NULL DEFAULT '',
    updated_by  uuid REFERENCES users (id) ON DELETE SET NULL,
    updated_at  timestamptz NOT NULL DEFAULT now()
);

INSERT INTO feature_flags (key, enabled, rollout, description)
VALUES
    ('absence_requests', true, 100, 'Заявки на отгул'),
    ('access_requests', true, 100, 'Заявки на доступы'),
    ('kb_citations', true, 100, 'Цитаты в ответах'),
    ('mentor_dashboard', true, 100, 'Панель наставника')
ON CONFLICT (key) DO NOTHING;

CREATE TABLE IF NOT EXISTS access_bundles (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bundle    text NOT NULL,
    system    text NOT NULL,
    reason    text NOT NULL DEFAULT '',
    approver  text NOT NULL DEFAULT '',
    position  integer NOT NULL DEFAULT 0,
    UNIQUE (bundle, system)
);

INSERT INTO access_bundles (bundle, system, reason, approver, position)
VALUES
    ('intern', 'VPN', 'Доступ в корпоративную сеть', 'IT-отдел', 1),
    ('intern', 'Jira', 'Работа с задачами команды', 'Наставник', 2),
    ('intern', 'База знаний', 'Документы команды', 'Наставник', 3),
    ('intern', 'Репозиторий', 'Доступ к коду команды', 'Наставник', 4),
    ('mentor', 'Jira', 'Ревью задач команды', 'Руководитель', 1),
    ('mentor', 'CI', 'Прогон пайплайнов', 'Руководитель', 2)
ON CONFLICT (bundle, system) DO NOTHING;
