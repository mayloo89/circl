ALTER TABLE profiles
    ADD COLUMN username VARCHAR(30) UNIQUE,
    ADD CONSTRAINT chk_profiles_username
        CHECK (username ~ '^[a-z0-9_]{3,30}$');

CREATE INDEX idx_profiles_username ON profiles (username) WHERE username IS NOT NULL;
