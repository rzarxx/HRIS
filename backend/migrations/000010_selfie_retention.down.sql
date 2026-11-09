DROP INDEX IF EXISTS idx_face_captures_retention;
ALTER TABLE face_captures DROP COLUMN selfie_deleted_at;
