-- Private albums with consented per-user sharing.
--
-- A user can group uploads into named albums and grant explicit access to
-- specific contacts. Photos are first-class uploads (category
-- 'album-private') that ride the standard moderation pipeline: hash-list
-- and heuristic detectors still block CSAM / NCII / malformed images, and
-- the NSFW classifier still blocks for now — the "tag-not-block" mode for
-- private surfaces lands in a follow-up PR.
--
-- The grant lifecycle covers three entry points:
--   - 'invite'  → owner adds a contact to the album      → status 'pending'  (viewer accepts)
--   - 'request' → viewer asks an owner for access         → status 'pending'  (owner approves)
--   - 'chat'    → owner shares the album in a DM message  → status 'active'  (implicit consent)
-- Either side can revoke an active grant; revoke is forward-only — past
-- views remain in the access log and the viewer's browser cache cannot be
-- invalidated retroactively (the UI must warn before revoking).
--
-- The access log (`private_album_views`) records every distinct photo a
-- viewer rendered, so the owner can audit who saw what.

CREATE TABLE private_albums (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    description     TEXT        NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    cover_upload_id UUID        REFERENCES uploads(id) ON DELETE SET NULL,
    photo_count     INTEGER     NOT NULL DEFAULT 0 CHECK (photo_count >= 0),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_private_albums_owner ON private_albums(owner_id, created_at DESC);

-- Links uploads to albums. PK on (album_id, upload_id) enforces uniqueness
-- — a given upload can appear in an album only once. Position carries the
-- owner-defined order (UI lets the owner drag-reorder later).
CREATE TABLE private_album_photos (
    album_id   UUID        NOT NULL REFERENCES private_albums(id) ON DELETE CASCADE,
    upload_id  UUID        NOT NULL REFERENCES uploads(id) ON DELETE CASCADE,
    position   INTEGER     NOT NULL DEFAULT 0,
    added_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (album_id, upload_id)
);

CREATE INDEX idx_private_album_photos_upload ON private_album_photos(upload_id);
CREATE INDEX idx_private_album_photos_position ON private_album_photos(album_id, position, added_at);

-- Grants control who can view an album.
--
-- The partial unique index ensures at most one open (pending or active)
-- grant exists per (album, grantee) pair — re-inviting the same person
-- after a revoke creates a new row, so the history of grants is preserved
-- for the audit trail without blocking the legitimate "re-grant" case.
CREATE TABLE private_album_grants (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    album_id      UUID        NOT NULL REFERENCES private_albums(id) ON DELETE CASCADE,
    granter_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    grantee_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status        TEXT        NOT NULL CHECK (status IN ('pending', 'active', 'denied', 'revoked')),
    source        TEXT        NOT NULL CHECK (source IN ('invite', 'request', 'chat')),
    requested_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    granted_at    TIMESTAMPTZ,
    revoked_at    TIMESTAMPTZ,
    CHECK (granter_id <> grantee_id)
);

CREATE UNIQUE INDEX uq_private_album_grants_open
    ON private_album_grants(album_id, grantee_id)
    WHERE status IN ('pending', 'active');

CREATE INDEX idx_private_album_grants_grantee
    ON private_album_grants(grantee_id, status, granted_at DESC);

CREATE INDEX idx_private_album_grants_album_status
    ON private_album_grants(album_id, status);

-- Append-only access log. One row per distinct view event so the owner
-- can audit who saw which photo when. NULL upload_id means a listing-only
-- view (the viewer opened the album but didn't fetch a specific photo).
CREATE TABLE private_album_views (
    id          BIGSERIAL   PRIMARY KEY,
    album_id    UUID        NOT NULL REFERENCES private_albums(id) ON DELETE CASCADE,
    viewer_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    upload_id   UUID        REFERENCES uploads(id) ON DELETE SET NULL,
    viewed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_private_album_views_album_viewed
    ON private_album_views(album_id, viewed_at DESC);

CREATE INDEX idx_private_album_views_viewer
    ON private_album_views(viewer_id, viewed_at DESC);
