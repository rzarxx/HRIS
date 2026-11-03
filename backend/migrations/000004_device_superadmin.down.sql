-- Nilai 'superadmin' tetap ada di enum user_role karena PostgreSQL tidak bisa menghapus nilai enum.
UPDATE users SET role = 'pejuang' WHERE role = 'superadmin';
DROP INDEX IF EXISTS idx_sessions_device;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_device_id_fkey;
DROP TABLE IF EXISTS devices;
DROP TYPE IF EXISTS device_status;
