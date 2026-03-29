ALTER TABLE profiles
    ADD COLUMN date_of_birth   DATE,
    ADD COLUMN gender          VARCHAR(50),
    ADD COLUMN location_text   VARCHAR(255),
    ADD COLUMN latitude        DOUBLE PRECISION,
    ADD COLUMN longitude       DOUBLE PRECISION,
    ADD COLUMN interests       TEXT[]       NOT NULL DEFAULT '{}';

-- Extend TSV to also index interests (weight C)
CREATE OR REPLACE FUNCTION refresh_profile_tsv()
RETURNS TRIGGER AS $$
BEGIN
    NEW.searchable_tsv :=
        setweight(to_tsvector('simple', coalesce(NEW.display_name, '')),                     'A') ||
        setweight(to_tsvector('simple', coalesce(NEW.bio, '')),                              'B') ||
        setweight(to_tsvector('simple', coalesce(array_to_string(NEW.interests, ' '), '')), 'C');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Re-create trigger to also fire on interests changes
DROP TRIGGER IF EXISTS trg_profiles_tsv ON profiles;
CREATE TRIGGER trg_profiles_tsv
    BEFORE INSERT OR UPDATE OF display_name, bio, interests ON profiles
    FOR EACH ROW EXECUTE FUNCTION refresh_profile_tsv();

CREATE TABLE IF NOT EXISTS profile_preferences (
    user_id           UUID        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    min_age           INT         CHECK (min_age >= 18),
    max_age           INT         CHECK (max_age <= 120),
    max_distance_km   INT         CHECK (max_distance_km > 0),
    gender_preference TEXT[]      NOT NULL DEFAULT '{}',
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
