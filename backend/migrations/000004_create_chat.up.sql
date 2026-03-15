CREATE TABLE rooms (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    type       TEXT        NOT NULL CHECK (type IN ('dm', 'group')),
    name       TEXT,
    -- Canonical key for DMs: min(userA, userB) || ':' || max(userA, userB).
    -- Enforces uniqueness at the DB level and enables conflict-free upserts.
    dm_key     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Partial unique index — only DM rooms carry a dm_key.
CREATE UNIQUE INDEX idx_rooms_dm_key ON rooms(dm_key) WHERE dm_key IS NOT NULL;

CREATE TABLE room_members (
    room_id      UUID        NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Tracks how far the user has read; used for unread-count queries.
    last_read_at TIMESTAMPTZ,
    PRIMARY KEY (room_id, user_id)
);

CREATE INDEX idx_room_members_user_id ON room_members(user_id);

CREATE TABLE messages (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id    UUID        NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    sender_id  UUID        NOT NULL REFERENCES users(id),
    type       TEXT        NOT NULL DEFAULT 'text' CHECK (type IN ('text', 'image', 'video', 'file')),
    content    TEXT        NOT NULL,
    -- Non-null means the message expires at this timestamp regardless of views.
    expires_at TIMESTAMPTZ,
    -- When true the message is hidden for each user after they view it once.
    view_once  BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_messages_room_created ON messages(room_id, created_at DESC);
-- Partial index to efficiently query expirable messages during cleanup sweeps.
CREATE INDEX idx_messages_expires_at ON messages(expires_at) WHERE expires_at IS NOT NULL;

-- Records which users have viewed a given message.
-- Dual purpose: read receipts for all messages + view-once deletion trigger.
CREATE TABLE message_views (
    message_id UUID        NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    viewed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (message_id, user_id)
);

CREATE TRIGGER trg_rooms_updated_at
    BEFORE UPDATE ON rooms
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
