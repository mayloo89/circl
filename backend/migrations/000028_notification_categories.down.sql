ALTER TABLE profile_preferences
    DROP COLUMN IF EXISTS notify_chat_messages,
    DROP COLUMN IF EXISTS notify_contact_requests,
    DROP COLUMN IF EXISTS notify_channel_mentions,
    DROP COLUMN IF EXISTS notify_system;
