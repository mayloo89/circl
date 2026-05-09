ALTER TABLE profile_preferences
    DROP COLUMN IF EXISTS hide_distance_from_non_contacts,
    DROP COLUMN IF EXISTS hide_presence,
    DROP COLUMN IF EXISTS hide_read_receipts,
    DROP COLUMN IF EXISTS hide_typing_indicator;
