DROP TABLE IF EXISTS attendance_attempts, attendance_challenges;
DROP INDEX IF EXISTS idx_att_open;
ALTER TABLE attendances
  DROP COLUMN IF EXISTS out_longitude,
  DROP COLUMN IF EXISTS out_latitude,
  DROP COLUMN IF EXISTS in_longitude,
  DROP COLUMN IF EXISTS in_latitude,
  DROP COLUMN IF EXISTS worked_minutes,
  DROP COLUMN IF EXISTS location_id,
  DROP COLUMN IF EXISTS work_mode;
