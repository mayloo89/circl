-- Add role column with default 'user'.
ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user';

-- Promote existing admins.
UPDATE users SET role = 'admin' WHERE is_admin = true;

-- Enforce valid roles.
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('user', 'admin', 'super_admin'));

-- Drop the old boolean column.
ALTER TABLE users DROP COLUMN is_admin;

-- Ensure 'purged' is included in the status constraint
-- (the anonymize worker sets status = 'purged').
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_status_check;
ALTER TABLE users ADD CONSTRAINT users_status_check
    CHECK (status IN ('active', 'suspended', 'banned', 'deleted', 'purged'));
