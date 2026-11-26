-- Notifikasi di aplikasi (D68). Saat ini hanya payslip_ready; tipe lain dari MASTER §6.2 menyusul.
CREATE TYPE notification_type AS ENUM ('payslip_ready');

CREATE TABLE notifications (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
  type        notification_type NOT NULL,
  ref_id      UUID,
  title       TEXT NOT NULL CHECK (char_length(title) <= 120),
  body        TEXT NOT NULL DEFAULT '' CHECK (char_length(body) <= 500),
  read_at     TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (employee_id, type, ref_id)
);
CREATE INDEX idx_notifications_employee ON notifications (employee_id, created_at DESC);
CREATE INDEX idx_notifications_unread ON notifications (employee_id) WHERE read_at IS NULL;
