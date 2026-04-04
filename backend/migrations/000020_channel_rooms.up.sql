-- Expand room type to include public channel rooms.
-- The check constraint was auto-named rooms_type_check by the original migration.
ALTER TABLE rooms DROP CONSTRAINT IF EXISTS rooms_type_check;
ALTER TABLE rooms ADD CONSTRAINT rooms_type_check CHECK (type IN ('dm', 'group', 'channel'));

-- Optional description visible in the channel directory.
ALTER TABLE rooms ADD COLUMN description TEXT;
