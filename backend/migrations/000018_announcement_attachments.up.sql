-- Lampiran pengumuman (D54). Berkasnya disimpan terenkripsi di SELFIE_DIR/announcements/; tabel ini hanya
-- mencatat metadata dan kunci penyimpanannya.
CREATE TABLE announcement_attachments (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  announcement_id UUID NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
  file_name       TEXT NOT NULL CHECK (length(file_name) BETWEEN 1 AND 120),
  content_type    TEXT NOT NULL CHECK (content_type IN ('application/pdf', 'image/jpeg', 'image/png')),
  size_bytes      INTEGER NOT NULL CHECK (size_bytes > 0),
  storage_key     TEXT NOT NULL UNIQUE,
  created_by      UUID REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_announcement_attachments_announcement ON announcement_attachments(announcement_id);
