ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;

ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('intern', 'mentor', 'manager', 'hr', 'admin'));

CREATE INDEX IF NOT EXISTS users_role_idx ON users (role);

CREATE INDEX IF NOT EXISTS onboarding_items_due_idx ON onboarding_items (due_on)
    WHERE due_on IS NOT NULL;
