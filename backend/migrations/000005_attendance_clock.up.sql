-- Fase 2: clock-in dasar. Tantangan sekali pakai, log setiap upaya, dan kolom absen (MASTER.md §5.4).
ALTER TABLE attendances
  ADD COLUMN work_mode      TEXT NOT NULL DEFAULT 'onsite' CHECK (work_mode IN ('onsite', 'wfa', 'field')),
  ADD COLUMN location_id    UUID REFERENCES office_locations(id),
  ADD COLUMN worked_minutes INT CHECK (worked_minutes >= 0),
  ADD COLUMN in_latitude    NUMERIC(9,6),
  ADD COLUMN in_longitude   NUMERIC(9,6),
  ADD COLUMN out_latitude   NUMERIC(9,6),
  ADD COLUMN out_longitude  NUMERIC(9,6);
CREATE INDEX idx_att_open ON attendances(employee_id, work_date DESC) WHERE clock_out_at IS NULL;

CREATE TABLE attendance_challenges (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id UUID NOT NULL REFERENCES employees(id),
  device_id   UUID NOT NULL REFERENCES devices(id),
  nonce       TEXT NOT NULL UNIQUE,
  purpose     TEXT NOT NULL CHECK (purpose IN ('clock_in', 'clock_out', 'face_enroll')),
  expires_at  TIMESTAMPTZ NOT NULL,
  used_at     TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE attendance_attempts (
  id           BIGSERIAL PRIMARY KEY,
  employee_id  UUID NOT NULL REFERENCES employees(id),
  device_id    UUID REFERENCES devices(id),
  challenge_id UUID REFERENCES attendance_challenges(id),
  purpose      TEXT NOT NULL,
  outcome      TEXT NOT NULL CHECK (outcome IN ('accepted', 'review', 'rejected')),
  reason_code  TEXT NOT NULL,
  risk_score   INT NOT NULL DEFAULT 0,
  risk_flags   JSONB NOT NULL DEFAULT '[]',
  latitude     NUMERIC(9,6),
  longitude    NUMERIC(9,6),
  ip           INET,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_attempts_emp_time ON attendance_attempts(employee_id, created_at DESC);
