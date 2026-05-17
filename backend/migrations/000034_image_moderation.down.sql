DROP TABLE IF EXISTS image_block_hashes;

DROP INDEX IF EXISTS idx_uploads_moderation_rejected;

ALTER TABLE uploads
    DROP COLUMN IF EXISTS moderated_at,
    DROP COLUMN IF EXISTS moderation_reason,
    DROP COLUMN IF EXISTS moderation_code,
    DROP COLUMN IF EXISTS moderation_status;
