-- Extensión para UUIDs
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Función genérica para mantener updated_at sincronizado
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE IF NOT EXISTS users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT,
    provider      TEXT        NOT NULL DEFAULT 'local'
                              CHECK (provider IN ('local', 'google', 'github')),
    status        TEXT        NOT NULL DEFAULT 'active'
                              CHECK (status IN ('active', 'suspended', 'deleted')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ,
    -- Si el provider es 'local', el password_hash es obligatorio
    CONSTRAINT chk_local_password CHECK (
        provider != 'local' OR password_hash IS NOT NULL
    )
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);

-- Trigger: actualiza updated_at automáticamente en cada UPDATE
CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
