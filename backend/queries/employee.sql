-- Filter scope: scope_all = true untuk SDM; selain itu dibatasi subdiv/divisi yang dipimpin.

-- name: ListEmployees :many
SELECT e.*, s.name AS subdivision_name, s.division_id, d.name AS division_name
FROM employees e
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
WHERE e.deleted_at IS NULL
  AND (@scope_all::boolean OR s.id = ANY(@scope_subdivision_ids::uuid[]) OR d.id = ANY(@scope_division_ids::uuid[]))
  AND (sqlc.narg(q)::text IS NULL
       OR e.full_name ILIKE '%' || sqlc.narg(q) || '%'
       OR e.nip ILIKE '%' || sqlc.narg(q) || '%'
       OR e.email ILIKE '%' || sqlc.narg(q) || '%'
       OR e.position ILIKE '%' || sqlc.narg(q) || '%')
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
       OR e.position ILIKE '%' || sqlc.narg(q) || '%')
  AND (sqlc.narg(division_id)::uuid IS NULL OR d.id = sqlc.narg(division_id))
  AND (sqlc.narg(subdivision_id)::uuid IS NULL OR s.id = sqlc.narg(subdivision_id))
  AND (sqlc.narg(status)::text IS NULL OR e.status = sqlc.narg(status));

-- name: GetEmployeeScoped :one
SELECT e.*, s.name AS subdivision_name, s.division_id, d.name AS division_name
FROM employees e
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
WHERE e.id = @id AND e.deleted_at IS NULL
  AND (@scope_all::boolean OR s.id = ANY(@scope_subdivision_ids::uuid[]) OR d.id = ANY(@scope_division_ids::uuid[]));

-- name: CreateEmployee :one
INSERT INTO employees (nip, full_name, email, phone, subdivision_id, position, join_date, base_salary, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id;

-- name: UpdateEmployee :execrows
UPDATE employees
SET nip = $2, full_name = $3, email = $4, phone = $5, subdivision_id = $6,
    position = $7, join_date = $8, base_salary = $9, status = $10, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: DeactivateEmployee :execrows
UPDATE employees SET status = 'inactive', exit_date = COALESCE(exit_date, @exit_date), updated_at = now()
WHERE id = @id AND deleted_at IS NULL AND status = 'active';

-- name: NextNipSequence :one
SELECT COALESCE(MAX(NULLIF(regexp_replace(right(nip, 4), '\D', '', 'g'), '')::int), 0)::int + 1 AS next_seq FROM employees;

-- name: GetEmployeeBaseSalary :one
SELECT base_salary FROM employees WHERE id = $1;
