CREATE TABLE face_remote_permissions (
  id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
  employee_id UUID PRIMARY KEY REFERENCES employees(id),
  approved_by UUID NOT NULL REFERENCES users(id),
  reason TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
