DROP TABLE IF EXISTS appeals;
DROP TABLE IF EXISTS age_verification_audit;

DROP INDEX IF EXISTS idx_reports_priority_status;
ALTER TABLE reports DROP COLUMN IF EXISTS priority;

ALTER TABLE reports DROP CONSTRAINT IF EXISTS reports_reason_check;
ALTER TABLE reports ADD CONSTRAINT reports_reason_check
    CHECK (reason IN ('harassment', 'spam', 'inappropriate_content', 'fake_profile', 'other'));
