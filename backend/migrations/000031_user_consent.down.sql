ALTER TABLE users
    DROP COLUMN IF EXISTS terms_accepted_at,
    DROP COLUMN IF EXISTS privacy_accepted_at,
    DROP COLUMN IF EXISTS accepted_policy_version;
