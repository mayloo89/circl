-- Profiles store only the center of a ~500 m cell, never the exact position a
-- client reported. New writes are snapped in profiles.SnapToCell; this
-- backfill applies the same formula to existing rows. It is irreversible by
-- design: the exact coordinates are discarded.

UPDATE profiles
   SET latitude = NULL, longitude = NULL
 WHERE (latitude IS NULL) <> (longitude IS NULL);

WITH rows AS (
    SELECT id,
           GREATEST(LEAST((FLOOR(latitude / 0.0045) + 0.5) * 0.0045, 90), -90) AS clat,
           longitude AS lng
      FROM profiles
     WHERE latitude IS NOT NULL AND longitude IS NOT NULL
), steps AS (
    SELECT id, clat, lng, 0.0045 / GREATEST(COS(RADIANS(clat)), 0.01) AS step
      FROM rows
)
UPDATE profiles p
   SET latitude  = ROUND(s.clat::numeric, 6)::double precision,
       longitude = ROUND(GREATEST(LEAST((FLOOR(s.lng / s.step) + 0.5) * s.step, 180), -180)::numeric, 6)::double precision
  FROM steps s
 WHERE p.id = s.id;
