-- Shift ganda dan setengah hari (D61). Satu Pejuang bisa punya beberapa shift dengan hari kerja yang
-- tidak bertabrakan, misalnya Senin-Jumat reguler dan Sabtu setengah hari; presensi memilih shift
-- sesuai hari. employees.default_shift_id tetap ada sebagai shift utama (urutan pertama).
ALTER TABLE shifts ADD COLUMN kind TEXT NOT NULL DEFAULT 'regular' CHECK (kind IN ('regular', 'half_day'));

CREATE TABLE employee_shifts (
  employee_id UUID NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
  shift_id    UUID NOT NULL REFERENCES shifts(id),
  PRIMARY KEY (employee_id, shift_id)
);
CREATE INDEX idx_employee_shifts_shift ON employee_shifts(shift_id);

INSERT INTO employee_shifts (employee_id, shift_id)
SELECT id, default_shift_id FROM employees WHERE default_shift_id IS NOT NULL;

-- Himpunan shift efektif: daftar shift ditambah shift utama, agar data yang hanya mengisi
-- default_shift_id (data lama, seed, alat lain) tetap terbaca sebagai satu shift.
CREATE VIEW employee_shift_set AS
SELECT employee_id, shift_id FROM employee_shifts
UNION
SELECT id AS employee_id, default_shift_id AS shift_id FROM employees WHERE default_shift_id IS NOT NULL;
