-- Extends messages.type CHECK to allow 'album_share', the new chat
-- message type introduced for the private-albums feature in PR #109.
-- Same pattern as migration 037 for uploads.category — the Go-side
-- `chat.MessageTypeAlbumShare` constant exists, this just aligns the DB.
ALTER TABLE messages DROP CONSTRAINT messages_type_check;
ALTER TABLE messages
    ADD CONSTRAINT messages_type_check
    CHECK (type = ANY (ARRAY['text'::text, 'image'::text, 'video'::text, 'file'::text, 'album_share'::text]));
