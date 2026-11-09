-- Simpan hash untuk mendeteksi replay setelah berkas selfie dihapus.
ALTER TABLE face_captures ADD COLUMN selfie_deleted_at TIMESTAMPTZ;
CREATE INDEX idx_face_captures_retention ON face_captures(created_at, id)
  WHERE selfie_deleted_at IS NULL;
