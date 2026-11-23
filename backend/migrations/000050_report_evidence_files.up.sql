-- Copies of reported media, held under the never-served "evidence/" storage
-- prefix. A row lives as long as its report keeps evidence: a dismissal or the
-- retention sweep deletes the object and the row together.
CREATE TABLE report_evidence_files (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id        UUID        NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    kind             TEXT        NOT NULL CHECK (kind IN ('message_media', 'profile_photo')),
    storage_key      TEXT        NOT NULL UNIQUE CHECK (storage_key LIKE 'evidence/%'),
    content_type     TEXT        NOT NULL,
    size_bytes       BIGINT      NOT NULL CHECK (size_bytes >= 0),
    -- Lets law enforcement or a hash database match the file without viewing it.
    sha256           TEXT        NOT NULL,
    -- The upload the copy was taken from. No FK: the original is expected to
    -- disappear (unsend, retention, account purge) while the copy remains.
    source_upload_id UUID,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_report_evidence_files_report ON report_evidence_files(report_id);
