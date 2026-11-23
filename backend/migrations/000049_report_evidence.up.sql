-- Reports keep their evidence even after the people or content involved are
-- gone. Account purge wipes profiles, retention hard-deletes DMs after three
-- months, and view-once/TTL messages are tombstoned, so a report must hold its
-- own copy of who was reported and what they said.

-- A user hard-delete must never take reports with it.
ALTER TABLE reports
    ALTER COLUMN reporter_id DROP NOT NULL,
    ALTER COLUMN reported_user_id DROP NOT NULL,
    DROP CONSTRAINT reports_reporter_id_fkey,
    DROP CONSTRAINT reports_reported_user_id_fkey,
    ADD CONSTRAINT reports_reporter_id_fkey
        FOREIGN KEY (reporter_id) REFERENCES users(id) ON DELETE SET NULL,
    ADD CONSTRAINT reports_reported_user_id_fkey
        FOREIGN KEY (reported_user_id) REFERENCES users(id) ON DELETE SET NULL;

-- Identity of the reported account as it was when the report was filed.
ALTER TABLE reports
    ADD COLUMN reported_username     TEXT,
    ADD COLUMN reported_display_name TEXT,
    ADD COLUMN reported_email        TEXT;

-- The reported message, copied at report time. No FK: the original may be
-- unsent, tombstoned, or swept by retention while the report is open.
ALTER TABLE reports
    ADD COLUMN message_id         UUID,
    ADD COLUMN message_room_id    UUID,
    ADD COLUMN message_type       TEXT,
    ADD COLUMN message_content    TEXT,
    ADD COLUMN message_sent_at    TIMESTAMPTZ,
    -- Set when a dismissal wipes the message copy.
    ADD COLUMN evidence_purged_at TIMESTAMPTZ;

-- One report per reporter per message.
CREATE UNIQUE INDEX idx_reports_reporter_message
    ON reports(reporter_id, message_id) WHERE message_id IS NOT NULL;

UPDATE reports r
   SET reported_username     = p.username,
       reported_display_name = p.display_name,
       reported_email        = u.email
  FROM users u
  LEFT JOIN profiles p ON p.user_id = u.id
 WHERE u.id = r.reported_user_id;
