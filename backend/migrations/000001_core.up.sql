-- Tabel inti Fase 0, gabungan QUERY.sql + MASTER.md Lampiran A.
-- Lokasi kantor dan kolom koordinat (PostGIS) menyusul di migrasi Fase 2.
-- Tabel modul lain (perangkat, wajah, pengajuan, payroll, SP) menyusul di fasenya masing-masing.

-- gen_random_uuid() bawaan PostgreSQL 13+, jadi tidak butuh ekstensi pgcrypto.

CREATE TYPE user_role AS ENUM ('pejuang','subdiv','kadiv','sdm');
CREATE TYPE att_status AS ENUM ('present','late','absent','leave','review');
CREATE TYPE ann_type AS ENUM ('popup','banner');

CREATE TABLE divisions (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code       TEXT UNIQUE NOT NULL,
  name       TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE subdivisions (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  division_id      UUID NOT NULL REFERENCES divisions(id),
  code             TEXT UNIQUE NOT NULL,
  name             TEXT NOT NULL,
  head_employee_id UUID,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE shifts (
  id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name                   TEXT NOT NULL,
  start_time             TIME NOT NULL,
  end_time               TIME NOT NULL,
  late_tolerance_minutes INT NOT NULL DEFAULT 0,
  work_days              SMALLINT[] NOT NULL DEFAULT '{1,2,3,4,5}' -- 1=Senin..7=Minggu
);

CREATE TABLE employees (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  nip              TEXT UNIQUE NOT NULL,
  full_name        TEXT NOT NULL,
  email            TEXT,
  phone            TEXT,
  subdivision_id   UUID NOT NULL REFERENCES subdivisions(id),
  position         TEXT,
  join_date        DATE,
  exit_date        DATE,
  base_salary      NUMERIC(14,2) NOT NULL DEFAULT 0,
  default_shift_id UUID REFERENCES shifts(id),
  status           TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','inactive')),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at       TIMESTAMPTZ
);
CREATE UNIQUE INDEX uq_employees_email ON employees (lower(email)) WHERE email IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_employees_subdiv ON employees(subdivision_id) WHERE deleted_at IS NULL;

ALTER TABLE subdivisions
  ADD CONSTRAINT fk_subdiv_head FOREIGN KEY (head_employee_id) REFERENCES employees(id);
ALTER TABLE divisions ADD COLUMN head_employee_id UUID REFERENCES employees(id);
CREATE INDEX idx_subdiv_division ON subdivisions(division_id);

CREATE TABLE users (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id          UUID UNIQUE NOT NULL REFERENCES employees(id),
  email                TEXT UNIQUE NOT NULL,
  password_hash        TEXT NOT NULL,
  role                 user_role NOT NULL DEFAULT 'pejuang',
  is_active            BOOLEAN NOT NULL DEFAULT true,
  failed_login_count   INT NOT NULL DEFAULT 0,
  locked_until         TIMESTAMPTZ,
  must_change_password BOOLEAN NOT NULL DEFAULT true,
  password_changed_at  TIMESTAMPTZ,
  last_login_at        TIMESTAMPTZ,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- device_id menjadi FK ke devices di migrasi Fase 2 (hanya sesi mobile yang mengisinya).
CREATE TABLE sessions (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id             UUID NOT NULL REFERENCES users(id),
  client              TEXT NOT NULL CHECK (client IN ('web','mobile')),
  device_id           UUID,
  refresh_hash        TEXT NOT NULL,
  prev_refresh_hash   TEXT,
  ip                  INET,
  user_agent          TEXT,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_used_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  idle_expires_at     TIMESTAMPTZ NOT NULL,
  absolute_expires_at TIMESTAMPTZ NOT NULL,
  revoked_at          TIMESTAMPTZ,
  revoke_reason       TEXT
);
CREATE UNIQUE INDEX uq_sessions_refresh ON sessions(refresh_hash);
CREATE INDEX idx_sessions_user ON sessions(user_id) WHERE revoked_at IS NULL;

CREATE TABLE attendances (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id    UUID NOT NULL REFERENCES employees(id),
  work_date      DATE NOT NULL,
  shift_id       UUID REFERENCES shifts(id),
  clock_in_at    TIMESTAMPTZ,
  clock_out_at   TIMESTAMPTZ,
  in_accuracy_m  NUMERIC(8,2),
  face_score     NUMERIC(5,4),
  risk_score     INT NOT NULL DEFAULT 0,
  risk_flags     JSONB NOT NULL DEFAULT '[]',
  status         att_status NOT NULL DEFAULT 'present',
  late_minutes   INT NOT NULL DEFAULT 0,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (employee_id, work_date)
);
CREATE INDEX idx_att_date ON attendances(work_date);
CREATE INDEX idx_att_review ON attendances(status) WHERE status = 'review';

-- image_upload_id menjadi FK ke uploads saat modul unggah dibuat.
CREATE TABLE announcements (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  title           TEXT NOT NULL,
  body            TEXT NOT NULL,
  type            ann_type NOT NULL DEFAULT 'banner',
  image_upload_id UUID,
  division_id     UUID REFERENCES divisions(id) ON DELETE SET NULL,
  starts_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  ends_at         TIMESTAMPTZ,
  is_active       BOOLEAN NOT NULL DEFAULT true,
  created_by      UUID REFERENCES users(id),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE audit_logs (
  id         BIGSERIAL PRIMARY KEY,
  user_id    UUID REFERENCES users(id),
  action     TEXT NOT NULL,
  entity     TEXT,
  entity_id  UUID,
  detail     JSONB,
  ip         INET,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_created ON audit_logs(created_at DESC);

-- Append-only (MASTER.md §5.10). Role hris_app dibuat oleh skrip penyiapan database.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'hris_app') THEN
    REVOKE UPDATE, DELETE ON audit_logs FROM hris_app;
  END IF;
END $$;
