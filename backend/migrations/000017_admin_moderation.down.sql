DROP TABLE IF EXISTS suspensions;
ALTER TABLE users DROP COLUMN IF EXISTS is_admin;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_status_check;
ALTER TABLE users ADD CONSTRAINT users_status_check
    CHECK (status IN ('active', 'suspended', 'deleted'));
