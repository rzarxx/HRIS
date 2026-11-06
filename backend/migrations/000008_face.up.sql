-- Fase 2: pendaftaran dan pencocokan wajah 1:1 (MASTER.md §5.5, F7).
-- Template disimpan terenkripsi di level aplikasi, bukan vektor mentah, jadi tidak perlu pgvector.
CREATE TABLE face_templates (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id    UUID NOT NULL REFERENCES employees(id),
  device_id      UUID REFERENCES devices(id),
  embedding_enc  BYTEA NOT NULL,
  key_id         TEXT NOT NULL,
  model_version  TEXT NOT NULL,
  dimension      INT NOT NULL CHECK (dimension > 0),
  is_active      BOOLEAN NOT NULL DEFAULT true,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  deactivated_at TIMESTAMPTZ,
  deactivated_by UUID REFERENCES users(id)
);
-- Satu Pejuang hanya punya satu template aktif.
CREATE UNIQUE INDEX idx_face_active ON face_templates(employee_id) WHERE is_active;

CREATE TABLE consents (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id UUID NOT NULL REFERENCES employees(id),
  type        TEXT NOT NULL CHECK (type IN ('biometric_face', 'location')),
  version     TEXT NOT NULL,
  ip          INET,
  accepted_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_consents_employee ON consents(employee_id, type, accepted_at DESC);
