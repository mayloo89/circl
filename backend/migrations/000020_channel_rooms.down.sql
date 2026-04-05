DELETE FROM rooms WHERE type = 'channel';
ALTER TABLE rooms DROP COLUMN IF EXISTS description;
ALTER TABLE rooms DROP CONSTRAINT IF EXISTS rooms_type_check;
ALTER TABLE rooms ADD CONSTRAINT rooms_type_check CHECK (type IN ('dm', 'group'));
