-- =====================================================
-- HRIS Pejuang - PostgreSQL schema + query penting
-- Butuh extension: postgis, vector (pgvector)
-- =====================================================

CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pgcrypto;  -- gen_random_uuid()

-- ---------- ENUM ----------
CREATE TYPE user_role       AS ENUM ('pejuang','subdiv','kadiv','sdm');
CREATE TYPE request_status  AS ENUM ('pending','approved','rejected','cancelled');
CREATE TYPE request_kind    AS ENUM ('leave','cash_advance','transfer');
CREATE TYPE device_status   AS ENUM ('pending','approved','revoked');
CREATE TYPE att_status      AS ENUM ('present','late','absent','leave','review');
CREATE TYPE ann_type        AS ENUM ('popup','banner');
CREATE TYPE sp_level        AS ENUM ('SP1','SP2','SP3');

-- ---------- STRUKTUR ORGANISASI ----------
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
  head_employee_id UUID,  -- FK ditambah setelah tabel employees
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE employees (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  nip            TEXT UNIQUE NOT NULL,
  full_name      TEXT NOT NULL,
  email          TEXT,
  phone          TEXT,
  subdivision_id UUID NOT NULL REFERENCES subdivisions(id),
  position       TEXT,
  join_date      DATE,
  base_salary    NUMERIC(14,2) NOT NULL DEFAULT 0,
  photo_url      TEXT,
  status         TEXT NOT NULL DEFAULT 'active',  -- active | inactive
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at     TIMESTAMPTZ
);

ALTER TABLE subdivisions
  ADD CONSTRAINT fk_subdiv_head FOREIGN KEY (head_employee_id) REFERENCES employees(id);

-- kepala divisi
ALTER TABLE divisions ADD COLUMN head_employee_id UUID REFERENCES employees(id);

CREATE TABLE users (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id   UUID UNIQUE NOT NULL REFERENCES employees(id),
  email         TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  role          user_role NOT NULL DEFAULT 'pejuang',
  is_active     BOOLEAN NOT NULL DEFAULT true,
  last_login_at TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_employees_subdiv ON employees(subdivision_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_subdiv_division  ON subdivisions(division_id);

-- ---------- PERANGKAT & WAJAH ----------
CREATE TABLE devices (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id UUID NOT NULL REFERENCES employees(id),
  device_hash TEXT NOT NULL,
  model       TEXT,
  os_version  TEXT,
  fcm_token   TEXT,
  status      device_status NOT NULL DEFAULT 'pending',
  approved_by UUID REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (employee_id, device_hash)
);

CREATE TABLE face_templates (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id UUID NOT NULL REFERENCES employees(id),
  embedding   vector(192) NOT NULL,   -- sesuaikan dimensi dgn model (128/192/512)
  is_active   BOOLEAN NOT NULL DEFAULT true,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------- GEOFENCE & SHIFT ----------
CREATE TABLE office_locations (
  id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name      TEXT NOT NULL,
  center    GEOGRAPHY(Point,4326) NOT NULL,
  radius_m  INT NOT NULL DEFAULT 100,
  polygon   GEOGRAPHY(Polygon,4326),   -- opsional, lebih presisi dari radius
  is_active BOOLEAN NOT NULL DEFAULT true
);
CREATE INDEX idx_office_center ON office_locations USING GIST (center);

CREATE TABLE employee_locations (
  employee_id UUID REFERENCES employees(id),
  location_id UUID REFERENCES office_locations(id),
  PRIMARY KEY (employee_id, location_id)
);

CREATE TABLE shifts (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name              TEXT NOT NULL,
  start_time        TIME NOT NULL,
  end_time          TIME NOT NULL,
  late_tolerance_min INT NOT NULL DEFAULT 0
);

-- ---------- PRESENSI ----------
CREATE TABLE attendances (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id       UUID NOT NULL REFERENCES employees(id),
  work_date         DATE NOT NULL,
  shift_id          UUID REFERENCES shifts(id),
  office_location_id UUID REFERENCES office_locations(id),
  device_id         UUID REFERENCES devices(id),
  clock_in_at       TIMESTAMPTZ,
  clock_out_at      TIMESTAMPTZ,
  in_location       GEOGRAPHY(Point,4326),
  out_location      GEOGRAPHY(Point,4326),
  in_accuracy_m     NUMERIC(8,2),
  face_score        NUMERIC(5,4),
  liveness_passed   BOOLEAN,
  integrity_verdict TEXT,
  risk_score        INT NOT NULL DEFAULT 0,
  risk_flags        JSONB NOT NULL DEFAULT '[]',
  selfie_url        TEXT,
  status            att_status NOT NULL DEFAULT 'present',
  late_minutes      INT NOT NULL DEFAULT 0,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (employee_id, work_date)
);
CREATE INDEX idx_att_date   ON attendances(work_date);
CREATE INDEX idx_att_review ON attendances(status) WHERE status = 'review';

-- ---------- PENGAJUAN ----------
CREATE TABLE leave_types (
  id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,           -- Cuti Tahunan, Sakit, Izin
  paid BOOLEAN NOT NULL DEFAULT true,
  max_days INT
);

CREATE TABLE leave_requests (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id   UUID NOT NULL REFERENCES employees(id),
  leave_type_id UUID NOT NULL REFERENCES leave_types(id),
  start_date    DATE NOT NULL,
  end_date      DATE NOT NULL,
  reason        TEXT,
  attachment_url TEXT,
  status        request_status NOT NULL DEFAULT 'pending',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (end_date >= start_date)
);

CREATE TABLE cash_advances (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id  UUID NOT NULL REFERENCES employees(id),
  amount       NUMERIC(14,2) NOT NULL CHECK (amount > 0),
  installments INT NOT NULL DEFAULT 1,
  reason       TEXT,
  status       request_status NOT NULL DEFAULT 'pending',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE transfer_requests (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id        UUID NOT NULL REFERENCES employees(id),
  from_subdivision_id UUID NOT NULL REFERENCES subdivisions(id),
  to_subdivision_id   UUID NOT NULL REFERENCES subdivisions(id),
  reason             TEXT,
  effective_date     DATE,
  status             request_status NOT NULL DEFAULT 'pending',
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (from_subdivision_id <> to_subdivision_id)
);

-- Mesin approval generik
CREATE TABLE approvals (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  request_type  request_kind NOT NULL,
  request_id    UUID NOT NULL,
  step_order    INT NOT NULL,
  approver_role user_role NOT NULL,
  approver_id   UUID REFERENCES users(id),   -- terisi saat diproses
  status        request_status NOT NULL DEFAULT 'pending',
  note          TEXT,
  acted_at      TIMESTAMPTZ,
  UNIQUE (request_type, request_id, step_order)
);
CREATE INDEX idx_approvals_pending ON approvals(approver_role, status) WHERE status = 'pending';

-- ---------- PAYROLL ----------
CREATE TABLE payroll_periods (
  id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  year   INT NOT NULL,
  month  INT NOT NULL CHECK (month BETWEEN 1 AND 12),
  status TEXT NOT NULL DEFAULT 'draft',   -- draft | final | paid
  UNIQUE (year, month)
);

CREATE TABLE payrolls (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  period_id   UUID NOT NULL REFERENCES payroll_periods(id),
  employee_id UUID NOT NULL REFERENCES employees(id),
  base_salary NUMERIC(14,2) NOT NULL,
  gross       NUMERIC(14,2) NOT NULL,
  deductions  NUMERIC(14,2) NOT NULL DEFAULT 0,
  net         NUMERIC(14,2) NOT NULL,
  slip_url    TEXT,
  UNIQUE (period_id, employee_id)
);

CREATE TABLE payroll_items (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  payroll_id UUID NOT NULL REFERENCES payrolls(id) ON DELETE CASCADE,
  kind       TEXT NOT NULL,      -- allowance | overtime | late | absent | cash_advance
  label      TEXT NOT NULL,
  amount     NUMERIC(14,2) NOT NULL   -- positif = tambah, negatif = potongan
);

-- ---------- SP, PENGUMUMAN, NOTIFIKASI, AUDIT ----------
CREATE TABLE warnings (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id UUID NOT NULL REFERENCES employees(id),
  level       sp_level NOT NULL,
  reason      TEXT NOT NULL,
  file_url    TEXT,
  issued_by   UUID REFERENCES users(id),
  issued_at   DATE NOT NULL DEFAULT current_date,
  valid_until DATE
);

CREATE TABLE announcements (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  title       TEXT NOT NULL,
  body        TEXT NOT NULL,
  type        ann_type NOT NULL DEFAULT 'banner',
  image_url   TEXT,
  division_id UUID REFERENCES divisions(id),  -- NULL = semua
  starts_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  ends_at     TIMESTAMPTZ,
  is_active   BOOLEAN NOT NULL DEFAULT true,
  created_by  UUID REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE announcement_reads (
  announcement_id UUID REFERENCES announcements(id) ON DELETE CASCADE,
  employee_id     UUID REFERENCES employees(id),
  read_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (announcement_id, employee_id)
);

CREATE TABLE notifications (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  employee_id UUID NOT NULL REFERENCES employees(id),
  title       TEXT NOT NULL,
  body        TEXT,
  is_read     BOOLEAN NOT NULL DEFAULT false,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
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

-- =====================================================
-- QUERY PENTING
-- =====================================================

-- 1. Grafik: jumlah Pejuang per subdiv
SELECT d.name AS divisi, s.name AS subdiv, COUNT(e.id) AS jumlah_pejuang
FROM subdivisions s
JOIN divisions d ON d.id = s.division_id
LEFT JOIN employees e ON e.subdivision_id = s.id
                     AND e.deleted_at IS NULL AND e.status = 'active'
GROUP BY d.name, s.name
ORDER BY d.name, s.name;

-- 2. Grafik: jumlah subdiv per divisi
SELECT d.name AS divisi, COUNT(s.id) AS jumlah_subdiv
FROM divisions d
LEFT JOIN subdivisions s ON s.division_id = d.id
GROUP BY d.name
ORDER BY d.name;

-- 3. Cek geofence (radius): $1=lng, $2=lat, $3=employee_id
SELECT ol.id, ol.name,
       ST_Distance(ol.center, ST_SetSRID(ST_MakePoint($1,$2),4326)::geography) AS jarak_m
FROM office_locations ol
JOIN employee_locations el ON el.location_id = ol.id AND el.employee_id = $3
WHERE ol.is_active
  AND ST_DWithin(ol.center, ST_SetSRID(ST_MakePoint($1,$2),4326)::geography, ol.radius_m)
ORDER BY jarak_m
LIMIT 1;

-- 4. Cek geofence (poligon)
SELECT ol.id FROM office_locations ol
WHERE ol.polygon IS NOT NULL
  AND ST_Covers(ol.polygon, ST_SetSRID(ST_MakePoint($1,$2),4326)::geography);

-- 5. Pencocokan wajah: $1=embedding hasil HP, $2=employee_id
--    <=> = cosine distance; similarity = 1 - distance
SELECT 1 - (embedding <=> $1::vector) AS similarity
FROM face_templates
WHERE employee_id = $2 AND is_active
ORDER BY embedding <=> $1::vector
LIMIT 1;

-- 6. Clock in (idempoten per hari)
INSERT INTO attendances
  (employee_id, work_date, shift_id, office_location_id, device_id, clock_in_at,
   in_location, in_accuracy_m, face_score, liveness_passed, integrity_verdict,
   risk_score, risk_flags, status, late_minutes)
VALUES
  ($1, current_date, $2, $3, $4, now(),
   ST_SetSRID(ST_MakePoint($5,$6),4326)::geography, $7, $8, $9, $10,
   $11, $12, $13, $14)
ON CONFLICT (employee_id, work_date) DO NOTHING
RETURNING id;

-- 7. Clock out
UPDATE attendances
SET clock_out_at = now(),
    out_location = ST_SetSRID(ST_MakePoint($2,$3),4326)::geography
WHERE employee_id = $1 AND work_date = current_date AND clock_out_at IS NULL;

-- 8. Rekap presensi bulanan per Pejuang: $1=tahun, $2=bulan
SELECT e.nip, e.full_name,
       COUNT(*) FILTER (WHERE a.status IN ('present','late')) AS hadir,
       COUNT(*) FILTER (WHERE a.status = 'late')              AS telat,
       COUNT(*) FILTER (WHERE a.status = 'leave')             AS cuti_izin,
       COUNT(*) FILTER (WHERE a.status = 'absent')            AS alpha,
       COALESCE(SUM(a.late_minutes),0)                        AS total_menit_telat
FROM employees e
LEFT JOIN attendances a ON a.employee_id = e.id
     AND EXTRACT(YEAR FROM a.work_date) = $1
     AND EXTRACT(MONTH FROM a.work_date) = $2
WHERE e.deleted_at IS NULL
GROUP BY e.id, e.nip, e.full_name
ORDER BY e.full_name;

-- 9. Laporan upah berdasarkan presensi (versi awal; rumus final menunggu keputusan SDM)
--    Contoh aturan: potongan alpha = gaji/26 per hari, telat = Rp 5.000 per kejadian
WITH rekap AS (
  SELECT e.id, e.nip, e.full_name, e.base_salary,
         COUNT(*) FILTER (WHERE a.status = 'absent') AS alpha,
         COUNT(*) FILTER (WHERE a.status = 'late')   AS telat
  FROM employees e
  LEFT JOIN attendances a ON a.employee_id = e.id
       AND EXTRACT(YEAR FROM a.work_date) = $1
       AND EXTRACT(MONTH FROM a.work_date) = $2
  WHERE e.deleted_at IS NULL
  GROUP BY e.id
)
SELECT nip, full_name, base_salary, alpha, telat,
       ROUND(alpha * base_salary / 26)  AS potongan_alpha,
       telat * 5000                     AS potongan_telat,
       base_salary - ROUND(alpha * base_salary / 26) - telat * 5000 AS upah_bersih
FROM rekap
ORDER BY full_name;

-- 10. Absen berisiko untuk direview SDM
SELECT a.id, e.full_name, a.work_date, a.risk_score, a.risk_flags, a.selfie_url
FROM attendances a JOIN employees e ON e.id = a.employee_id
WHERE a.status = 'review'
ORDER BY a.risk_score DESC, a.work_date DESC;

-- 11. Antrian approval untuk satu role (hanya step yang gilirannya)
SELECT ap.*
FROM approvals ap
WHERE ap.approver_role = $1
  AND ap.status = 'pending'
  AND NOT EXISTS (
    SELECT 1 FROM approvals prev
    WHERE prev.request_type = ap.request_type
      AND prev.request_id   = ap.request_id
      AND prev.step_order   < ap.step_order
      AND prev.status <> 'approved'
  )
ORDER BY ap.id;

-- 12. Pengumuman aktif untuk seorang Pejuang yang belum dibaca
SELECT an.*
FROM announcements an
JOIN employees e      ON e.id = $1
JOIN subdivisions s   ON s.id = e.subdivision_id
LEFT JOIN announcement_reads r ON r.announcement_id = an.id AND r.employee_id = e.id
WHERE an.is_active
  AND now() >= an.starts_at
  AND (an.ends_at IS NULL OR now() <= an.ends_at)
  AND (an.division_id IS NULL OR an.division_id = s.division_id)
  AND r.employee_id IS NULL
ORDER BY an.starts_at DESC;

-- 13. Terapkan mutasi yang sudah disetujui penuh
UPDATE employees e
SET subdivision_id = t.to_subdivision_id, updated_at = now()
FROM transfer_requests t
WHERE t.employee_id = e.id
  AND t.status = 'approved'
  AND t.effective_date <= current_date
  AND e.subdivision_id = t.from_subdivision_id;

-- 14. Data dummy untuk demo Jumat
INSERT INTO divisions (code,name) VALUES
 ('OPS','Operasional'),('KEU','Keuangan'),('IT','Teknologi Informasi');
INSERT INTO subdivisions (division_id,code,name)
SELECT id,'OPS-LAP','Lapangan' FROM divisions WHERE code='OPS' UNION ALL
SELECT id,'OPS-GDG','Gudang'   FROM divisions WHERE code='OPS' UNION ALL
SELECT id,'KEU-AKT','Akuntansi' FROM divisions WHERE code='KEU' UNION ALL
SELECT id,'IT-DEV','Pengembangan' FROM divisions WHERE code='IT';
