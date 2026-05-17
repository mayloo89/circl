-- Image moderation pipeline.
--
-- `moderation_status` is the explicit moderation outcome, separate from the
-- upload lifecycle (`status`). A committed upload that was later flagged is
-- `status='failed'` (the storage object is gone) plus
-- `moderation_status='rejected'` with the reason. A non-image upload is
-- marked 'skipped' so the admin queue does not get flooded with them.
--
-- Hash-list table is the local store the moderator queries first. Source is
-- recorded so a future StopNCII / PhotoDNA integration appends rows from
-- those feeds without overwriting locally-curated entries.

ALTER TABLE uploads
    ADD COLUMN IF NOT EXISTS moderation_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (moderation_status IN ('pending', 'approved', 'rejected', 'skipped')),
    ADD COLUMN IF NOT EXISTS moderation_code   TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS moderation_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS moderated_at      TIMESTAMPTZ;

-- Newest-rejected-first is the only sort the admin queue cares about.
CREATE INDEX IF NOT EXISTS idx_uploads_moderation_rejected
    ON uploads(moderated_at DESC)
    WHERE moderation_status = 'rejected';

CREATE TABLE image_block_hashes (
    hash        TEXT        PRIMARY KEY,
    source      TEXT        NOT NULL,           -- 'local' | 'stopncii' | 'photodna' | ...
    reason      TEXT        NOT NULL,
    added_by    UUID        REFERENCES users(id) ON DELETE SET NULL,
    added_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_image_block_hashes_source ON image_block_hashes(source);
