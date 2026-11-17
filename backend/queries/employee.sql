-- Filter scope: scope_all = true untuk SDM; selain itu dibatasi subdiv/divisi yang dipimpin.

-- name: ListEmployees :many
SELECT e.*, s.name AS subdivision_name, s.division_id, d.name AS division_name, d.branch_id, b.name AS branch_name,
       sh.name AS default_shift_name,
       EXISTS (SELECT 1 FROM face_templates f WHERE f.employee_id = e.id AND f.is_active) AS face_enrolled
FROM employees e
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
JOIN branches b ON b.id = d.branch_id
LEFT JOIN shifts sh ON sh.id = e.default_shift_id
WHERE e.deleted_at IS NULL
  AND (@scope_all::boolean OR s.id = ANY(@scope_subdivision_ids::uuid[]) OR d.id = ANY(@scope_division_ids::uuid[]))
  AND (sqlc.narg(q)::text IS NULL
       OR e.full_name ILIKE '%' || sqlc.narg(q) || '%'
       OR e.nip ILIKE '%' || sqlc.narg(q) || '%'
       OR e.email ILIKE '%' || sqlc.narg(q) || '%'
       OR e.phone ILIKE '%' || sqlc.narg(q) || '%'
       OR e.position ILIKE '%' || sqlc.narg(q) || '%')
  AND (sqlc.narg(branch_id)::uuid IS NULL OR d.branch_id = sqlc.narg(branch_id))
  AND (sqlc.narg(division_id)::uuid IS NULL OR d.id = sqlc.narg(division_id))
  AND (sqlc.narg(subdivision_id)::uuid IS NULL OR s.id = sqlc.narg(subdivision_id))
  AND (sqlc.narg(status)::text IS NULL OR e.status = sqlc.narg(status))
ORDER BY e.full_name, e.id
LIMIT @page_limit OFFSET @page_offset;

-- name: CountEmployees :one
SELECT count(*)
FROM employees e
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
WHERE e.deleted_at IS NULL
  AND (@scope_all::boolean OR s.id = ANY(@scope_subdivision_ids::uuid[]) OR d.id = ANY(@scope_division_ids::uuid[]))
  AND (sqlc.narg(q)::text IS NULL
       OR e.full_name ILIKE '%' || sqlc.narg(q) || '%'
       OR e.nip ILIKE '%' || sqlc.narg(q) || '%'
       OR e.email ILIKE '%' || sqlc.narg(q) || '%'
       OR e.phone ILIKE '%' || sqlc.narg(q) || '%'
       OR e.position ILIKE '%' || sqlc.narg(q) || '%')
  AND (sqlc.narg(branch_id)::uuid IS NULL OR d.branch_id = sqlc.narg(branch_id))
  AND (sqlc.narg(division_id)::uuid IS NULL OR d.id = sqlc.narg(division_id))
  AND (sqlc.narg(subdivision_id)::uuid IS NULL OR s.id = sqlc.narg(subdivision_id))
  AND (sqlc.narg(status)::text IS NULL OR e.status = sqlc.narg(status));

-- name: GetEmployeeScoped :one
SELECT e.*, s.name AS subdivision_name, s.division_id, d.name AS division_name, d.branch_id, b.name AS branch_name,
       sh.name AS default_shift_name,
       EXISTS (SELECT 1 FROM face_templates f WHERE f.employee_id = e.id AND f.is_active) AS face_enrolled
FROM employees e
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
JOIN branches b ON b.id = d.branch_id
LEFT JOIN shifts sh ON sh.id = e.default_shift_id
WHERE e.id = @id AND e.deleted_at IS NULL
  AND (@scope_all::boolean OR s.id = ANY(@scope_subdivision_ids::uuid[]) OR d.id = ANY(@scope_division_ids::uuid[]));

-- name: CreateEmployee :one
INSERT INTO employees (nip, full_name, email, phone, subdivision_id, position, join_date, base_salary, status, default_shift_id,
                       gender, marital_status, has_children, children_count)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id;

-- name: UpdateEmployee :execrows
UPDATE employees
SET nip = $2, full_name = $3, email = $4, phone = $5, subdivision_id = $6,
    position = $7, join_date = $8, base_salary = $9, status = $10, default_shift_id = $11,
    gender = $12, marital_status = $13, has_children = $14, children_count = $15, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: DeactivateEmployee :execrows
UPDATE employees SET status = 'inactive', exit_date = COALESCE(exit_date, @exit_date), updated_at = now()
WHERE id = @id AND deleted_at IS NULL AND status = 'active';

-- Impor massal: NIPY yang sudah ada diperbarui, bukan dibuat ulang.
-- name: ListEmployeesByNips :many
SELECT id, upper(nip)::text AS nip FROM employees
WHERE deleted_at IS NULL AND upper(nip) = ANY(@nips::text[]);

-- name: NextNipSequence :one
SELECT COALESCE(MAX(NULLIF(regexp_replace(right(nip, 4), '\D', '', 'g'), '')::int), 0)::int + 1 AS next_seq FROM employees;

-- name: GetEmployeeBaseSalary :one
SELECT base_salary FROM employees WHERE id = $1;

-- Data anak hanya untuk SDM, super admin, dan Pejuang sendiri (D53).
-- name: ListEmployeeChildren :many
SELECT * FROM employee_children WHERE employee_id = $1 ORDER BY child_order;

-- name: DeleteEmployeeChildren :exec
DELETE FROM employee_children WHERE employee_id = $1;

-- name: InsertEmployeeChild :exec
INSERT INTO employee_children (employee_id, child_order, full_name, gender, birth_place, birth_date, relation, education, school_name)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);
