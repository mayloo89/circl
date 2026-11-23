DROP INDEX IF EXISTS idx_reports_reporter_message;

ALTER TABLE reports
    DROP COLUMN evidence_purged_at,
    DROP COLUMN message_sent_at,
    DROP COLUMN message_content,
    DROP COLUMN message_type,
    DROP COLUMN message_room_id,
    DROP COLUMN message_id,
    DROP COLUMN reported_email,
    DROP COLUMN reported_display_name,
    DROP COLUMN reported_username;

-- Reports whose accounts were hard-deleted can't satisfy NOT NULL again.
DELETE FROM reports WHERE reporter_id IS NULL OR reported_user_id IS NULL;

ALTER TABLE reports
    DROP CONSTRAINT reports_reporter_id_fkey,
    DROP CONSTRAINT reports_reported_user_id_fkey,
    ADD CONSTRAINT reports_reporter_id_fkey
        FOREIGN KEY (reporter_id) REFERENCES users(id) ON DELETE CASCADE,
    ADD CONSTRAINT reports_reported_user_id_fkey
        FOREIGN KEY (reported_user_id) REFERENCES users(id) ON DELETE CASCADE,
    ALTER COLUMN reporter_id SET NOT NULL,
    ALTER COLUMN reported_user_id SET NOT NULL;
