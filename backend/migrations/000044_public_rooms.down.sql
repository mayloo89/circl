-- Guest messages (sender_id IS NULL) cannot satisfy the restored NOT NULL, so
-- they are removed before re-applying the constraint.
ALTER TABLE messages DROP COLUMN IF EXISTS sender_label;
DELETE FROM messages WHERE sender_id IS NULL;
ALTER TABLE messages ALTER COLUMN sender_id SET NOT NULL;

DROP INDEX IF EXISTS idx_rooms_public;
DROP INDEX IF EXISTS idx_rooms_public_name;
ALTER TABLE rooms DROP COLUMN IF EXISTS visibility;
ALTER TABLE rooms DROP CONSTRAINT IF EXISTS rooms_type_check;
ALTER TABLE rooms ADD CONSTRAINT rooms_type_check CHECK (type IN ('dm', 'group', 'channel'));
