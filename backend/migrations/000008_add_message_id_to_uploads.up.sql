ALTER TABLE uploads ADD COLUMN message_id UUID REFERENCES messages(id) ON DELETE CASCADE;
CREATE INDEX idx_uploads_message_id ON uploads(message_id) WHERE message_id IS NOT NULL;
