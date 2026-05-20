CREATE TABLE IF NOT EXISTS private_album_views (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    album_id   uuid        NOT NULL REFERENCES private_albums(id) ON DELETE CASCADE,
    viewer_id  uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    upload_id  uuid        REFERENCES uploads(id) ON DELETE SET NULL,
    viewed_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ON private_album_views (album_id, viewed_at DESC);
CREATE INDEX ON private_album_views (viewer_id, viewed_at DESC);
