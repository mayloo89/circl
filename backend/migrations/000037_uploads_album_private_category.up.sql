-- Extends the uploads.category CHECK constraint to allow 'album-private',
-- the new category introduced for the private-albums feature in PR #109.
-- The Go-side storage.ParseCategory + ValidateUpload already accept it; this
-- migration aligns the DB so INSERTs don't trip the original three-value
-- CHECK that predated the feature.
ALTER TABLE uploads DROP CONSTRAINT uploads_category_check;
ALTER TABLE uploads
    ADD CONSTRAINT uploads_category_check
    CHECK (category = ANY (ARRAY['avatar'::text, 'chat-attachment'::text, 'gallery'::text, 'album-private'::text]));
