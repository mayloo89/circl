ALTER TABLE profiles
    ADD COLUMN date_of_birth   DATE,
    ADD COLUMN gender          VARCHAR(50),
    ADD COLUMN location_text   VARCHAR(255),
    ADD COLUMN latitude        DOUBLE PRECISION,
    ADD COLUMN longitude       DOUBLE PRECISION;

-- Shared interest catalogue
CREATE TABLE interests (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(50) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_interests_name ON interests (name varchar_pattern_ops);

-- Per-user interest membership
CREATE TABLE profile_interests (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    interest_id UUID NOT NULL REFERENCES interests(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, interest_id)
);

-- Extend TSV to also index interests (weight C) via subquery
CREATE OR REPLACE FUNCTION refresh_profile_tsv()
RETURNS TRIGGER AS $$
BEGIN
    NEW.searchable_tsv :=
        setweight(to_tsvector('simple', coalesce(NEW.display_name, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(NEW.bio, '')),          'B') ||
        setweight(to_tsvector('simple', (
            SELECT coalesce(string_agg(i.name, ' '), '')
              FROM profile_interests pi
              JOIN interests i ON i.id = pi.interest_id
             WHERE pi.user_id = NEW.user_id
        )), 'C');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_profiles_tsv ON profiles;
CREATE TRIGGER trg_profiles_tsv
    BEFORE INSERT OR UPDATE OF display_name, bio ON profiles
    FOR EACH ROW EXECUTE FUNCTION refresh_profile_tsv();

-- Keep TSV fresh when profile_interests changes
CREATE OR REPLACE FUNCTION refresh_profile_tsv_on_interests()
RETURNS TRIGGER AS $$
DECLARE
    v_user_id UUID;
BEGIN
    v_user_id := COALESCE(NEW.user_id, OLD.user_id);
    UPDATE profiles
       SET searchable_tsv =
           setweight(to_tsvector('simple', coalesce(display_name, '')), 'A') ||
           setweight(to_tsvector('simple', coalesce(bio, '')),          'B') ||
           setweight(to_tsvector('simple', (
               SELECT coalesce(string_agg(i.name, ' '), '')
                 FROM profile_interests pi
                 JOIN interests i ON i.id = pi.interest_id
                WHERE pi.user_id = v_user_id
           )), 'C')
     WHERE user_id = v_user_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_profile_interests_tsv
    AFTER INSERT OR DELETE ON profile_interests
    FOR EACH ROW EXECUTE FUNCTION refresh_profile_tsv_on_interests();

CREATE TABLE IF NOT EXISTS profile_preferences (
    user_id           UUID        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    min_age           INT         CHECK (min_age >= 18),
    max_age           INT         CHECK (max_age <= 120),
    max_distance_km   INT         CHECK (max_distance_km > 0),
    gender_preference TEXT[]      NOT NULL DEFAULT '{}',
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
