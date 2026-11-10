-- Fase 4: mesin approval generik dan cuti/izin (MASTER.md F12, F13, D46, D47, D49).
CREATE TYPE request_kind AS ENUM ('leave', 'cash_advance', 'transfer');
CREATE TYPE request_status AS ENUM ('pending', 'approved', 'rejected', 'cancelled');
CREATE TYPE approval_step_status AS ENUM ('waiting', 'pending', 'approved', 'rejected', 'skipped');
-- statutory: hak cuti tahunan UU Ketenagakerjaan, custom: jatah per tahun dari SDM, none: tanpa saldo.
CREATE TYPE leave_quota_kind AS ENUM ('statutory', 'custom', 'none');

-- Alur per jenis pengajuan, diatur SDM (approval_flow:manage).
CREATE TABLE approval_flow_steps (
  request_type  request_kind NOT NULL,
  step_order    SMALLINT NOT NULL CHECK (step_order BETWEEN 1 AND 5),
  approver_role user_role NOT NULL CHECK (approver_role IN ('subdiv', 'kadiv', 'sdm')),
  PRIMARY KEY (request_type, step_order)
);
-- D46: cuti dan izin disetujui kepala divisi lalu SDM.
INSERT INTO approval_flow_steps (request_type, step_order, approver_role) VALUES
  ('leave', 1, 'kadiv'),
  ('leave', 2, 'sdm');

-- Snapshot langkah per pengajuan. assigned_employee_id NULL berarti pool role sdm.
CREATE TABLE approvals (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  request_type          request_kind NOT NULL,
  request_id            UUID NOT NULL,
  requester_employee_id UUID NOT NULL REFERENCES employees(id),
  step_order            SMALLINT NOT NULL,
  approver_role         user_role NOT NULL,
  assigned_employee_id  UUID REFERENCES employees(id),
  status                approval_step_status NOT NULL DEFAULT 'waiting',
  note                  TEXT,
  acted_by              UUID REFERENCES users(id),
  acted_at              TIMESTAMPTZ,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (request_type, request_id, step_order)
);
CREATE INDEX idx_approvals_pending_assigned ON approvals(assigned_employee_id) WHERE status = 'pending';
CREATE INDEX idx_approvals_pending_pool ON approvals(approver_role) WHERE status = 'pending' AND assigned_employee_id IS NULL;

CREATE TABLE leave_types (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code              TEXT NOT NULL UNIQUE CHECK (code ~ '^[a-z][a-z0-9_]{1,31}$'),
  name              TEXT NOT NULL,
  paid              BOOLEAN NOT NULL DEFAULT true,
  quota_kind        leave_quota_kind NOT NULL DEFAULT 'none',
  annual_quota_days SMALLINT CHECK (annual_quota_days BETWEEN 1 AND 366),
  is_active         BOOLEAN NOT NULL DEFAULT true,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((quota_kind = 'custom') = (annual_quota_days IS NOT NULL))
);
-- D47: jenis awal; SDM dapat menambah jenis dengan jatah sendiri atau mengubah yang ada.
INSERT INTO leave_types (code, name, quota_kind) VALUES
  ('annual', 'Cuti Tahunan', 'statutory'),
  ('sick', 'Sakit', 'none'),
  ('permit', 'Izin', 'none');

CREATE TABLE leave_requests (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id   UUID NOT NULL REFERENCES employees(id),
  leave_type_id UUID NOT NULL REFERENCES leave_types(id),
  start_date    DATE NOT NULL,
  end_date      DATE NOT NULL,
  days          SMALLINT NOT NULL CHECK (days > 0), -- hari kerja dalam rentang, dihitung server
  reason        TEXT NOT NULL,
  status        request_status NOT NULL DEFAULT 'pending',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  decided_at    TIMESTAMPTZ,
  CHECK (end_date >= start_date)
);
CREATE INDEX idx_leave_requests_employee ON leave_requests(employee_id, start_date DESC);

-- D49: role approver mengikuti penunjukan kepala. Role sdm dan superadmin tidak diubah.
UPDATE users u SET role = 'kadiv'
FROM divisions d
WHERE d.head_employee_id = u.employee_id AND u.role IN ('pejuang', 'subdiv');

UPDATE users u SET role = 'subdiv'
FROM subdivisions s
WHERE s.head_employee_id = u.employee_id AND u.role = 'pejuang';
