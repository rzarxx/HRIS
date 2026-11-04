-- Fase 2: izin WFA, hybrid, dan jam fleksibel per Pejuang (MASTER.md §5.4.2, D23, D26).
CREATE TABLE employee_work_arrangements (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id    UUID NOT NULL REFERENCES employees(id),
  mode           TEXT NOT NULL CHECK (mode IN ('wfa', 'hybrid')),
  flexible_hours BOOLEAN NOT NULL DEFAULT false,
  starts_on      DATE NOT NULL,
  ends_on        DATE,
  weekdays       SMALLINT[] NOT NULL DEFAULT '{}' CHECK (weekdays <@ ARRAY[1,2,3,4,5,6,7]::SMALLINT[]), -- kosong = setiap hari, 1=Senin..7=Minggu
  reason         TEXT NOT NULL,
  created_by     UUID NOT NULL REFERENCES users(id),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (ends_on IS NULL OR ends_on >= starts_on)
);
CREATE INDEX idx_work_arrangements_employee ON employee_work_arrangements(employee_id, starts_on DESC);

-- Disimpan per absen karena perhitungan lembur nanti tidak boleh bergantung pada izin yang sudah dihapus (D29).
ALTER TABLE attendances ADD COLUMN flexible_hours BOOLEAN NOT NULL DEFAULT false;
