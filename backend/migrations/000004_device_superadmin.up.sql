-- Fase 2: role super admin (D25) dan device binding (MASTER.md §5.3).
ALTER TYPE user_role ADD VALUE IF NOT EXISTS 'superadmin';

CREATE TYPE device_status AS ENUM ('pending', 'approved', 'revoked');

CREATE TABLE devices (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id    UUID NOT NULL REFERENCES employees(id),
  device_hash    TEXT NOT NULL,
  model          TEXT,
  os_version     TEXT,
  fcm_token      TEXT,
  status         device_status NOT NULL DEFAULT 'pending',
  approved_by    UUID REFERENCES users(id),
  approved_at    TIMESTAMPTZ,
  revoked_by     UUID REFERENCES users(id),
  revoked_at     TIMESTAMPTZ,
  revoked_reason TEXT,
  last_seen_at   TIMESTAMPTZ,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (employee_id, device_hash)
);
CREATE INDEX idx_devices_status ON devices(status, created_at DESC);
-- Satu akun, satu perangkat approved. Penjaga terakhir bila ada balapan di kode.
CREATE UNIQUE INDEX uq_devices_one_approved ON devices(employee_id) WHERE status = 'approved';

ALTER TABLE sessions
  ADD CONSTRAINT sessions_device_id_fkey FOREIGN KEY (device_id) REFERENCES devices(id);
CREATE INDEX idx_sessions_device ON sessions(device_id) WHERE device_id IS NOT NULL;
