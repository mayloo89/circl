-- Down migration is a no-op: merging gallery and avatar cannot be safely
-- reversed (the new ordering is the source of truth; restoring the old split
-- would require knowing which row was the "original" avatar, which is lost).
SELECT 1;
