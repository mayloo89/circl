-- Quarantine disposition for CSAM-class moderation hits.
--
-- A confirmed CSAM match must NOT be deleted: the legal duty is to preserve
-- the object (for the reporting/retention window) while making it unservable
-- and unreachable from the ordinary admin review UI. This is distinct from:
--   - 'rejected' + file_retained = TRUE  (NSFW/heuristic, admin-reviewable)
--   - 'rejected' + file_retained = FALSE (operator block-list / NCII, purged)
--
-- 'quarantined' uploads have their original storage object removed and a copy
-- written under a restricted prefix recorded in `moderation_quarantine_key`.
ALTER TABLE uploads
    DROP CONSTRAINT IF EXISTS uploads_moderation_status_check;

ALTER TABLE uploads
    ADD CONSTRAINT uploads_moderation_status_check
        CHECK (moderation_status IN ('pending', 'approved', 'rejected', 'skipped', 'quarantined'));

ALTER TABLE uploads
    ADD COLUMN IF NOT EXISTS moderation_quarantine_key TEXT;
