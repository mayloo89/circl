-- Reverts the CHECK constraint to its pre-private-albums state. Note: if
-- any rows with category='album-private' exist, the constraint re-add will
-- fail — that's intentional, you'd have to clean those rows up first.
ALTER TABLE uploads DROP CONSTRAINT uploads_category_check;
ALTER TABLE uploads
    ADD CONSTRAINT uploads_category_check
    CHECK (category = ANY (ARRAY['avatar'::text, 'chat-attachment'::text, 'gallery'::text]));
