DROP TABLE IF EXISTS profile_preferences;

-- Restore original TSV function (without interests)
CREATE OR REPLACE FUNCTION refresh_profile_tsv()
RETURNS TRIGGER AS $$
BEGIN
    NEW.searchable_tsv :=
        setweight(to_tsvector('simple', coalesce(NEW.display_name, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(NEW.bio, '')),          'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_profiles_tsv ON profiles;
CREATE TRIGGER trg_profiles_tsv
    BEFORE INSERT OR UPDATE OF display_name, bio ON profiles
    FOR EACH ROW EXECUTE FUNCTION refresh_profile_tsv();

ALTER TABLE profiles
    DROP COLUMN IF EXISTS interests,
    DROP COLUMN IF EXISTS longitude,
    DROP COLUMN IF EXISTS latitude,
    DROP COLUMN IF EXISTS location_text,
    DROP COLUMN IF EXISTS gender,
    DROP COLUMN IF EXISTS date_of_birth;
