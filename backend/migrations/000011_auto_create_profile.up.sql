CREATE OR REPLACE FUNCTION create_profile_for_new_user()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO profiles (user_id, display_name, bio, avatar_url)
    VALUES (NEW.id, '', '', '')
    ON CONFLICT (user_id) DO NOTHING;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_create_profile_on_register
AFTER INSERT ON users
FOR EACH ROW EXECUTE FUNCTION create_profile_for_new_user();
