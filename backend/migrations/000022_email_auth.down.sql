DROP TABLE IF EXISTS email_verifications;
DROP TABLE IF EXISTS password_resets;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;
