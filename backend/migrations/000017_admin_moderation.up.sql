ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_status_check;
ALTER TABLE users ADD CONSTRAINT users_status_check
    CHECK (status IN ('active', 'suspended', 'banned', 'deleted'));

CREATE TABLE suspensions (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    suspended_until TIMESTAMPTZ,
    reason          TEXT        NOT NULL,
    created_by      UUID        NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_suspensions_user ON suspensions(user_id, created_at DESC);
