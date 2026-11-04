-- Fase 2: keputusan Review Absen (MASTER.md F11).
ALTER TABLE attendances
  ADD COLUMN review_by   UUID REFERENCES users(id),
  ADD COLUMN reviewed_at TIMESTAMPTZ,
  ADD COLUMN review_note TEXT;
