ALTER TABLE private_albums
    ADD COLUMN cover_upload_id UUID REFERENCES uploads(id) ON DELETE SET NULL;
