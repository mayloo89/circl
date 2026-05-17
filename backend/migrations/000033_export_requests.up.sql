-- Per-user data export requests (Habeas Data / GDPR Art. 20).
-- One row per request; the asynq worker writes the zip to storage and stamps
-- `storage_key` + `token_hash` + `expires_at` when the build finishes.
-- The download endpoint hashes the path token and looks it up here, so the
-- plaintext token only exists in the email we send.
--
-- The unique partial index keeps a user from queueing a second build while
-- one is already in flight — a deliberate stop-gap until the per-day rate
-- limiter has been long enough in place to be the single source of truth.

CREATE TABLE export_requests (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status          TEXT        NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','processing','ready','failed','expired')),
    storage_key     TEXT,
    token_hash      TEXT        UNIQUE,
    error           TEXT        NOT NULL DEFAULT '',
    requested_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    downloaded_at   TIMESTAMPTZ
);

CREATE INDEX idx_export_requests_user
    ON export_requests(user_id, requested_at DESC);

CREATE UNIQUE INDEX idx_export_requests_one_open_per_user
    ON export_requests(user_id)
    WHERE status IN ('pending','processing');
