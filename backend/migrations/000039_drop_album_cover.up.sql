-- Drop the cover_upload_id column. Albums no longer have a designated
-- cover photo — the UI represents them with a generic glyph + name.
-- The FK constraint goes with the column drop.
ALTER TABLE private_albums DROP COLUMN cover_upload_id;
