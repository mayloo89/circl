ALTER TABLE uploads
    DROP COLUMN IF EXISTS moderation_quarantine_key;

ALTER TABLE uploads
    DROP CONSTRAINT IF EXISTS uploads_moderation_status_check;

ALTER TABLE uploads
    ADD CONSTRAINT uploads_moderation_status_check
        CHECK (moderation_status IN ('pending', 'approved', 'rejected', 'skipped'));
