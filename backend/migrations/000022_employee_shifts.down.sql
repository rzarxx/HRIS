DROP VIEW IF EXISTS employee_shift_set;
DROP TABLE IF EXISTS employee_shifts;
ALTER TABLE shifts DROP COLUMN IF EXISTS kind;
