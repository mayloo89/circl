CREATE TABLE IF NOT EXISTS uploads (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    storage_key  TEXT        NOT NULL UNIQUE,
    filename     TEXT        NOT NULL,
    content_type TEXT        NOT NULL,
    size_bytes   BIGINT      NOT NULL DEFAULT 0,
    category     TEXT        NOT NULL CHECK (category IN ('avatar', 'chat-attachment')),
    status       TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'committed', 'failed')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    committed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_uploads_user_id ON uploads(user_id);
CREATE INDEX IF NOT EXISTS idx_uploads_storage_key ON uploads(storage_key);
CREATE INDEX IF NOT EXISTS idx_uploads_pending ON uploads(created_at) WHERE status = 'pending';
