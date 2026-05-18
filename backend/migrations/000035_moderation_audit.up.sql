-- Moderation audit columns and review-retention flag.
--
-- `moderation_score` carries the classifier confidence (0.0 - 1.0) for NSFW
-- rejections so an admin can see *how* explicit the model thought the image
-- was, not just that it tripped the threshold. Null for non-classifier
-- rejections (hash_match, heuristic) and for approved uploads.
--
-- `moderation_categories` holds the per-region labels the classifier
-- returned (e.g. {FEMALE_BREAST_EXPOSED, BUTTOCKS_EXPOSED}). Same null
-- semantics as the score.
--
-- `moderation_file_retained` distinguishes "rejected but storage object kept
-- for admin review" from "rejected and purged". Hash-list matches are
-- always purged immediately (legal posture: hash lists target CSAM / NCII,
-- which must not be retained). NSFW and heuristic rejections retain the
-- file for MODERATION_REJECTED_RETENTION_DAYS (default 30) so admins can
-- verify false positives, then the cleanup worker purges them and flips
-- the flag back to FALSE.
ALTER TABLE uploads
    ADD COLUMN IF NOT EXISTS moderation_score         DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS moderation_categories    TEXT[],
    ADD COLUMN IF NOT EXISTS moderation_file_retained BOOLEAN NOT NULL DEFAULT FALSE;

-- The cleanup worker scans this index to find files past their retention
-- window. Partial index keeps it tiny — only rejected uploads with the
-- file still around appear here.
CREATE INDEX IF NOT EXISTS idx_uploads_moderation_retained
    ON uploads(moderated_at)
    WHERE moderation_status = 'rejected' AND moderation_file_retained = TRUE;
