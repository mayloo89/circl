ALTER TABLE profiles
    ADD COLUMN IF NOT EXISTS looking_for_gender  TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS looking_for_age_min INT,
    ADD COLUMN IF NOT EXISTS looking_for_age_max INT;
