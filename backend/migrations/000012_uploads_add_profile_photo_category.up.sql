ALTER TABLE uploads
    DROP CONSTRAINT IF EXISTS uploads_category_check,
    ADD CONSTRAINT uploads_category_check
        CHECK (category IN ('avatar', 'chat-attachment', 'gallery'));
