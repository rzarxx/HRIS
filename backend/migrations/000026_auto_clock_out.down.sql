DROP INDEX IF EXISTS idx_attendances_auto_pending;
ALTER TABLE attendances
  DROP COLUMN IF EXISTS adjust_note,
  DROP COLUMN IF EXISTS adjusted_at,
  DROP COLUMN IF EXISTS adjusted_by,
  DROP COLUMN IF EXISTS auto_clock_out;
