-- Pengumuman sampai ke aplikasi: jeda baca per pengumuman dan catatan buka/baca per Pejuang (D27).
ALTER TABLE announcements
  ADD COLUMN min_read_seconds SMALLINT NOT NULL DEFAULT 10
    CHECK (min_read_seconds BETWEEN 0 AND 120);

CREATE TABLE announcement_reads (
  announcement_id UUID NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
  employee_id     UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
  -- Dicatat sekali saat isi pertama kali dibuka; timer baca dihitung dari sini di server.
  opened_at       TIMESTAMPTZ NOT NULL,
  read_at         TIMESTAMPTZ,
  PRIMARY KEY (announcement_id, employee_id),
  CHECK (read_at IS NULL OR read_at >= opened_at)
);
CREATE INDEX idx_announcement_reads_employee ON announcement_reads(employee_id);
