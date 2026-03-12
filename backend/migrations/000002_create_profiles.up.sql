CREATE TABLE IF NOT EXISTS profiles (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID        NOT NULL UNIQUE,
    display_name   TEXT        NOT NULL,
    bio            TEXT        NOT NULL DEFAULT '',
    avatar_url     TEXT,
    is_searchable  BOOLEAN     NOT NULL DEFAULT true,
    searchable_tsv tsvector,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_profiles_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- Índices de búsqueda
CREATE INDEX IF NOT EXISTS idx_profiles_tsv          ON profiles USING GIN (searchable_tsv);
CREATE INDEX IF NOT EXISTS idx_profiles_display_name ON profiles (display_name);
-- Permite filtrar rápido usuarios buscables
CREATE INDEX IF NOT EXISTS idx_profiles_searchable   ON profiles (is_searchable) WHERE is_searchable = true;

-- Trigger: actualiza updated_at automáticamente
CREATE TRIGGER trg_profiles_updated_at
    BEFORE UPDATE ON profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Función que recalcula searchable_tsv a partir de display_name y bio
CREATE OR REPLACE FUNCTION refresh_profile_tsv()
RETURNS TRIGGER AS $$
BEGIN
    NEW.searchable_tsv :=
        setweight(to_tsvector('simple', coalesce(NEW.display_name, '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(NEW.bio, '')),          'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger: recalcula el tsvector en INSERT y UPDATE
CREATE TRIGGER trg_profiles_tsv
    BEFORE INSERT OR UPDATE OF display_name, bio ON profiles
    FOR EACH ROW EXECUTE FUNCTION refresh_profile_tsv();
