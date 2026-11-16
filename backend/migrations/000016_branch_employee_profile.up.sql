-- D53: cabang, data Pejuang sesuai formulir yayasan, data anak, kata sandi awal, dan login NIPY/email/WA.

-- Cabang punya struktur sendiri (divisi, subdiv, atasan). Divisi lama masuk cabang Pusat.
CREATE TABLE branches (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code       TEXT NOT NULL UNIQUE CHECK (code ~ '^[A-Z0-9-]{2,20}$'),
  name       TEXT NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO branches (code, name) VALUES ('PUSAT', 'Pusat');

ALTER TABLE divisions ADD COLUMN branch_id UUID REFERENCES branches(id);
UPDATE divisions SET branch_id = (SELECT id FROM branches WHERE code = 'PUSAT');
ALTER TABLE divisions ALTER COLUMN branch_id SET NOT NULL;
CREATE INDEX idx_divisions_branch ON divisions(branch_id);

-- Kode divisi unik per cabang, kode subdiv unik per divisi: struktur cabang boleh sama.
ALTER TABLE divisions DROP CONSTRAINT divisions_code_key;
ALTER TABLE divisions ADD CONSTRAINT uq_divisions_branch_code UNIQUE (branch_id, code);
ALTER TABLE subdivisions DROP CONSTRAINT subdivisions_code_key;
ALTER TABLE subdivisions ADD CONSTRAINT uq_subdivisions_division_code UNIQUE (division_id, code);

ALTER TABLE employees
  ADD COLUMN gender         TEXT CHECK (gender IN ('L', 'P')),
  ADD COLUMN marital_status TEXT CHECK (marital_status IN ('belum_menikah', 'menikah', 'cerai_hidup', 'cerai_mati')),
  ADD COLUMN has_children   BOOLEAN,
  ADD COLUMN children_count SMALLINT CHECK (children_count BETWEEN 0 AND 30);

-- Nomor WA dipakai untuk login, jadi disimpan dalam satu bentuk baku (62…) dan unik.
UPDATE employees SET phone = regexp_replace(phone, '\D', '', 'g') WHERE phone IS NOT NULL;
UPDATE employees SET phone = '62' || substr(phone, 2) WHERE phone ~ '^08';
UPDATE employees SET phone = NULL WHERE phone IS NOT NULL AND phone !~ '^628[0-9]{7,12}$';
-- Data development bisa memuat nomor kembar; yang lebih baru dikosongkan agar indeks unik bisa dibuat.
UPDATE employees e SET phone = NULL
WHERE phone IS NOT NULL AND EXISTS (
  SELECT 1 FROM employees o
  WHERE o.phone = e.phone AND o.deleted_at IS NULL AND (o.created_at, o.id) < (e.created_at, e.id)
);
ALTER TABLE employees ADD CONSTRAINT chk_employees_phone CHECK (phone ~ '^628[0-9]{7,12}$');
CREATE UNIQUE INDEX uq_employees_phone ON employees (phone) WHERE phone IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE employee_children (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
  child_order SMALLINT NOT NULL CHECK (child_order BETWEEN 1 AND 30),
  full_name   TEXT NOT NULL,
  gender      TEXT CHECK (gender IN ('L', 'P')),
  birth_place TEXT,
  birth_date  DATE,
  relation    TEXT CHECK (relation IN ('kandung', 'angkat', 'tiri')),
  education   TEXT CHECK (education IN ('belum_sekolah', 'paud', 'tk_ra', 'sd_mi', 'smp_mts', 'sma_sederajat',
                                        'perguruan_tinggi', 'pondok_pesantren')),
  school_name TEXT,
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (employee_id, child_order)
);

-- Email tidak lagi wajib: login bisa memakai NIPY atau nomor WA.
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
