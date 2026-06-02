-- Channels are broadcast-only: their messages are no longer persisted.
-- Purge rows accumulated by the previous unconditional-save behavior.
DELETE FROM messages
WHERE room_id IN (SELECT id FROM rooms WHERE type = 'channel');
