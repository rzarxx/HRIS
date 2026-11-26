-- Komponen ubar diatur SDM lewat panel (D66, melanjutkan D29): pendapatan dan potongan dengan metode
-- hitung tetap dari daftar ini, tanpa rumus bebas. Komponen tidak dihapus, hanya dinonaktifkan, karena
-- periode lama menyimpan salinan komponen saat generate.
CREATE TYPE payroll_component_kind AS ENUM ('earning', 'deduction');

CREATE TYPE payroll_calc_method AS ENUM (
  'fixed',                 -- nominal sama untuk semua Pejuang
  'per_employee',          -- nominal per Pejuang (employee_payroll_components)
  'per_present_day',       -- rate x hari hadir
  'per_late',              -- rate x kali telat
  'per_late_minute',       -- rate x menit telat
  'per_absent_day',        -- rate x hari alpha, atau ubar pokok / divisor x hari alpha bila rate kosong
  'per_unpaid_leave_day',  -- seperti per_absent_day untuk cuti tidak dibayar
  'percent_of_base',       -- persen x ubar pokok
  'percent_of_gross',      -- persen x total pendapatan
  'per_unit',              -- rate x kuantitas yang diisi SDM per periode (mis. jam lembur)
  'manual'                 -- nominal diisi SDM per periode
);

CREATE TABLE payroll_components (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code         TEXT NOT NULL UNIQUE CHECK (code ~ '^[A-Z][A-Z0-9_]{1,19}$'),
  name         TEXT NOT NULL CHECK (char_length(name) BETWEEN 2 AND 80),
  kind         payroll_component_kind NOT NULL,
  method       payroll_calc_method NOT NULL,
  rate         NUMERIC(14,2) CHECK (rate IS NULL OR rate >= 0),
  percent      NUMERIC(6,3) CHECK (percent IS NULL OR (percent >= 0 AND percent <= 100)),
  divisor      NUMERIC(6,2) CHECK (divisor IS NULL OR divisor > 0),
  active       BOOLEAN NOT NULL DEFAULT true,
  show_on_slip BOOLEAN NOT NULL DEFAULT true,
  sort_order   INTEGER NOT NULL DEFAULT 0,
  note         TEXT NOT NULL DEFAULT '' CHECK (char_length(note) <= 300),
  updated_by   UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Nilai komponen per_employee untuk tiap Pejuang, mis. tunjangan jabatan.
CREATE TABLE employee_payroll_components (
  employee_id  UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
  component_id UUID NOT NULL REFERENCES payroll_components(id),
  amount       NUMERIC(14,2) NOT NULL CHECK (amount >= 0),
  updated_by   UUID REFERENCES users(id) ON DELETE SET NULL,
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (employee_id, component_id)
);
CREATE INDEX idx_employee_payroll_components_component ON employee_payroll_components(component_id);
