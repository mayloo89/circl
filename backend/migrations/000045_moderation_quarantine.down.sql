ALTER TABLE uploads
    DROP COLUMN IF EXISTS moderation_quarantine_key;

ALTER TABLE uploads
    DROP CONSTRAINT IF EXISTS uploads_moderation_status_check;

-- Map any quarantined rows to a legacy-allowed value before re-adding the
-- old constraint, otherwise ADD CONSTRAINT would fail and block rollback.
UPDATE uploads SET moderation_status = 'rejected' WHERE moderation_status = 'quarantined';

ALTER TABLE uploads
    ADD CONSTRAINT uploads_moderation_status_check
        CHECK (moderation_status IN ('pending', 'approved', 'rejected', 'skipped'));
