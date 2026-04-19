ALTER TABLE profile_preferences
    ADD COLUMN IF NOT EXISTS locale TEXT NOT NULL DEFAULT 'es';
