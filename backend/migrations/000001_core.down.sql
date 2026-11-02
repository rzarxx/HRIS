DROP TABLE IF EXISTS audit_logs, announcements, attendances, sessions, users CASCADE;
ALTER TABLE IF EXISTS divisions DROP COLUMN IF EXISTS head_employee_id;
ALTER TABLE IF EXISTS subdivisions DROP CONSTRAINT IF EXISTS fk_subdiv_head;
DROP TABLE IF EXISTS employees, shifts, subdivisions, divisions CASCADE;
DROP TYPE IF EXISTS ann_type, att_status, user_role;
