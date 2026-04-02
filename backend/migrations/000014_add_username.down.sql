DROP INDEX IF EXISTS idx_profiles_username;

ALTER TABLE profiles
    DROP CONSTRAINT IF EXISTS chk_profiles_username,
    DROP COLUMN IF EXISTS username;
