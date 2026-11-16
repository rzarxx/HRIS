-- Akun tanpa email tidak bisa dipertahankan setelah kolom kembali wajib.
DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE email IS NULL);
UPDATE users SET email = 'tanpa-email-' || id || '@invalid' WHERE email IS NULL;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;

DROP TABLE employee_children;

DROP INDEX uq_employees_phone;
ALTER TABLE employees DROP CONSTRAINT chk_employees_phone;
ALTER TABLE employees
  DROP COLUMN children_count,
  DROP COLUMN has_children,
  DROP COLUMN marital_status,
  DROP COLUMN gender;

ALTER TABLE subdivisions DROP CONSTRAINT uq_subdivisions_division_code;
ALTER TABLE subdivisions ADD CONSTRAINT subdivisions_code_key UNIQUE (code);
ALTER TABLE divisions DROP CONSTRAINT uq_divisions_branch_code;
ALTER TABLE divisions ADD CONSTRAINT divisions_code_key UNIQUE (code);

DROP INDEX idx_divisions_branch;
ALTER TABLE divisions DROP COLUMN branch_id;
DROP TABLE branches;
