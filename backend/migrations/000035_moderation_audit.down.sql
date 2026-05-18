DROP INDEX IF EXISTS idx_uploads_moderation_retained;

ALTER TABLE uploads
    DROP COLUMN IF EXISTS moderation_file_retained,
    DROP COLUMN IF EXISTS moderation_categories,
    DROP COLUMN IF EXISTS moderation_score;
