ALTER TABLE profiles
    DROP COLUMN IF EXISTS looking_for_gender,
    DROP COLUMN IF EXISTS looking_for_age_min,
    DROP COLUMN IF EXISTS looking_for_age_max;
