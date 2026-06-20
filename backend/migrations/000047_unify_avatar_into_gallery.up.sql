-- Unify avatar into the gallery so profile_photos is the single source of
-- truth and profiles.avatar_url is a server-maintained mirror of position 0.
--
-- Three idempotent backfill cases:
--   1. Avatar exists but is not yet in the gallery → insert it at position 0.
--   2. Avatar already in gallery but not first → repack so it leads.
--   3. No avatar but gallery exists → mirror the current first photo to avatar_url.

BEGIN;

-- Increase max-photos constant guard (6 → 7) so existing full galleries can
-- absorb a distinct avatar without dropping a photo.
-- (The application code MaxProfilePhotos is updated in the same PR.)

-- Case 1: avatar set, not yet in gallery → insert at position 0.
-- First shift all existing photos up by 1 to make room.
WITH needs_insert AS (
    SELECT p.user_id, p.avatar_url
    FROM profiles p
    WHERE p.avatar_url IS NOT NULL
      AND p.avatar_url <> ''
      AND NOT EXISTS (
          SELECT 1 FROM profile_photos pp
          WHERE pp.user_id = p.user_id AND pp.url = p.avatar_url
      )
)
UPDATE profile_photos pp
   SET position = pp.position + 1
  FROM needs_insert ni
 WHERE pp.user_id = ni.user_id;

INSERT INTO profile_photos (user_id, url, position)
SELECT p.user_id, p.avatar_url, 0
FROM profiles p
WHERE p.avatar_url IS NOT NULL
  AND p.avatar_url <> ''
  AND NOT EXISTS (
      SELECT 1 FROM profile_photos pp
      WHERE pp.user_id = p.user_id AND pp.url = p.avatar_url
  );

-- Case 2: avatar already in gallery but not at position 0 → repack so it leads.
-- Identify users whose avatar URL is in the gallery but at a non-zero position.
WITH avatar_not_first AS (
    SELECT p.user_id, pp.id AS avatar_photo_id, pp.position AS avatar_pos
    FROM profiles p
    JOIN profile_photos pp ON pp.user_id = p.user_id AND pp.url = p.avatar_url
    WHERE p.avatar_url IS NOT NULL
      AND p.avatar_url <> ''
      AND pp.position <> 0
),
-- Assign new positions: avatar row gets 0; others get their relative rank + 1.
new_positions AS (
    SELECT
        ph.id,
        ph.user_id,
        CASE
            WHEN ph.id = anf.avatar_photo_id THEN 0
            ELSE (ROW_NUMBER() OVER (
                    PARTITION BY ph.user_id
                    ORDER BY ph.position, ph.created_at
                  ))::int
        END AS new_pos
    FROM profile_photos ph
    JOIN avatar_not_first anf ON anf.user_id = ph.user_id
)
UPDATE profile_photos pp
   SET position = np.new_pos
  FROM new_positions np
 WHERE pp.id = np.id;

-- Case 3: no avatar but gallery exists → set avatar_url to the first gallery photo.
UPDATE profiles p
   SET avatar_url = (
       SELECT url FROM profile_photos pp
       WHERE pp.user_id = p.user_id
       ORDER BY pp.position, pp.created_at
       LIMIT 1
   ),
   updated_at = now()
WHERE (p.avatar_url IS NULL OR p.avatar_url = '')
  AND EXISTS (SELECT 1 FROM profile_photos pp WHERE pp.user_id = p.user_id);

COMMIT;
