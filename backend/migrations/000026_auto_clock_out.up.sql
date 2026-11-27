-- Pulang otomatis (D69): presensi masuk tanpa pulang yang batasnya lewat ditutup job harian dengan jam akhir
-- shift. SDM mengecek hasilnya di Review Presensi; adjusted_* mencatat siapa yang mengecek atau mengoreksi.
ALTER TABLE attendances
  ADD COLUMN auto_clock_out BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN adjusted_by    UUID REFERENCES users(id),
  ADD COLUMN adjusted_at    TIMESTAMPTZ,
  ADD COLUMN adjust_note    TEXT CHECK (char_length(adjust_note) <= 500);

CREATE INDEX idx_attendances_auto_pending ON attendances (work_date DESC) WHERE auto_clock_out AND adjusted_at IS NULL;
