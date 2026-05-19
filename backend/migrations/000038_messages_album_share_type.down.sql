ALTER TABLE messages DROP CONSTRAINT messages_type_check;
ALTER TABLE messages
    ADD CONSTRAINT messages_type_check
    CHECK (type = ANY (ARRAY['text'::text, 'image'::text, 'video'::text, 'file'::text]));
