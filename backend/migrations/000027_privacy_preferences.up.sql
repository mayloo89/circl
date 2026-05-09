ALTER TABLE profile_preferences
    ADD COLUMN IF NOT EXISTS hide_distance_from_non_contacts BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS hide_presence                   BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS hide_read_receipts              BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS hide_typing_indicator           BOOLEAN NOT NULL DEFAULT FALSE;
