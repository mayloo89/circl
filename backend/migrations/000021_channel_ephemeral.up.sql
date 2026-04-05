-- Channels are now ephemeral: membership is the active WebSocket connection.
-- Remove any persisted room_members rows for channel rooms.
DELETE FROM room_members
WHERE room_id IN (SELECT id FROM rooms WHERE type = 'channel');
