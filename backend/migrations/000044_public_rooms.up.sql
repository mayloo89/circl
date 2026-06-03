-- Expand room type to include public rooms (guest-accessible chat rooms).
ALTER TABLE rooms DROP CONSTRAINT IF EXISTS rooms_type_check;
ALTER TABLE rooms ADD CONSTRAINT rooms_type_check CHECK (type IN ('dm', 'group', 'channel', 'public'));

-- Public rooms carry a visibility flag so future members-only rooms can
-- coexist under the same type. Currently only 'public' is allowed.
ALTER TABLE rooms ADD COLUMN visibility TEXT;

-- Partial unique index: public rooms require unique names to avoid confusion
-- in the guest-facing directory. Other room types allow duplicate names.
CREATE UNIQUE INDEX idx_rooms_public_name ON rooms(name) WHERE type = 'public';

-- Index for listing public rooms ordered by creation date.
CREATE INDEX idx_rooms_public ON rooms(created_at DESC) WHERE type = 'public' AND visibility = 'public';

-- Guest senders have no users row, so sender_id must be nullable and the
-- ephemeral nickname is stored in sender_label. The FK on sender_id tolerates
-- NULL; registered messages keep sender_id and leave sender_label NULL.
ALTER TABLE messages ALTER COLUMN sender_id DROP NOT NULL;
ALTER TABLE messages ADD COLUMN sender_label TEXT;
