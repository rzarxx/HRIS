-- Periode ubar dengan maker-checker (D67). Setiap periode menyimpan salinan komponen dan setelan saat
-- generate (rules_snapshot), dan setiap slip menyimpan salinan identitas, presensi, dan item, sehingga
-- perubahan data master tidak mengubah periode lama.
CREATE TYPE payroll_status AS ENUM ('draft', 'pending_approval', 'final', 'paid');

CREATE TABLE payroll_periods (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  year           SMALLINT NOT NULL CHECK (year BETWEEN 2000 AND 2100),
  month          SMALLINT NOT NULL CHECK (month BETWEEN 1 AND 12),
  start_date     DATE NOT NULL,
  end_date       DATE NOT NULL, -- inklusif
  status         payroll_status NOT NULL DEFAULT 'draft',
  rules_snapshot JSONB,
  created_by     UUID NOT NULL REFERENCES users(id),
  generated_by   UUID REFERENCES users(id),
  generated_at   TIMESTAMPTZ,
  submitted_by   UUID REFERENCES users(id),
  submitted_at   TIMESTAMPTZ,
  finalized_by   UUID REFERENCES users(id),
  finalized_at   TIMESTAMPTZ,
  paid_by        UUID REFERENCES users(id),
  paid_at        TIMESTAMPTZ,
  reject_note    TEXT CHECK (char_length(reject_note) <= 500),
  rejected_by    UUID REFERENCES users(id),
  rejected_at    TIMESTAMPTZ,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (year, month),
  CHECK (end_date >= start_date AND end_date - start_date < 62)
);
CREATE INDEX idx_payroll_periods_range ON payroll_periods (start_date, end_date);

CREATE TABLE payrolls (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  period_id       UUID NOT NULL REFERENCES payroll_periods(id) ON DELETE CASCADE,
  employee_id     UUID NOT NULL REFERENCES employees(id),
  nip             TEXT NOT NULL,
  full_name       TEXT NOT NULL,
  division        TEXT NOT NULL,
  subdivision     TEXT NOT NULL,
  position        TEXT NOT NULL DEFAULT '',
  base_salary     NUMERIC(14,2) NOT NULL,
  gross           NUMERIC(14,2) NOT NULL,
  deductions      NUMERIC(14,2) NOT NULL,
  net             NUMERIC(14,2) NOT NULL,
  net_clamped     BOOLEAN NOT NULL DEFAULT false,
  attendance      JSONB NOT NULL,
  employee_values JSONB NOT NULL DEFAULT '{}', -- nilai komponen per_employee saat generate
  published_at    TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (period_id, employee_id)
);
CREATE INDEX idx_payrolls_employee ON payrolls (employee_id);

CREATE TABLE payroll_items (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  payroll_id     UUID NOT NULL REFERENCES payrolls(id) ON DELETE CASCADE,
  position       SMALLINT NOT NULL,
  component_code TEXT NOT NULL,
  name           TEXT NOT NULL,
  kind           payroll_component_kind NOT NULL,
  quantity       NUMERIC(12,2),
  rate           NUMERIC(14,2),
  amount         NUMERIC(14,2) NOT NULL CHECK (amount >= 0),
  show_on_slip   BOOLEAN NOT NULL DEFAULT true,
  manual         BOOLEAN NOT NULL DEFAULT false,
  UNIQUE (payroll_id, position)
);

-- Isian SDM per periode untuk komponen per_unit (quantity) dan manual (amount).
CREATE TABLE payroll_inputs (
  period_id    UUID NOT NULL REFERENCES payroll_periods(id) ON DELETE CASCADE,
  employee_id  UUID NOT NULL REFERENCES employees(id),
  component_id UUID NOT NULL REFERENCES payroll_components(id),
  quantity     NUMERIC(12,2) CHECK (quantity >= 0),
  amount       NUMERIC(14,2) CHECK (amount >= 0),
  note         TEXT NOT NULL DEFAULT '' CHECK (char_length(note) <= 300),
  updated_by   UUID REFERENCES users(id),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (period_id, employee_id, component_id),
  CHECK (quantity IS NOT NULL OR amount IS NOT NULL)
);
