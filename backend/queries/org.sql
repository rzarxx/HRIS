-- name: ListDivisions :many
SELECT d.*,
       h.full_name AS head_name,
       (SELECT count(*) FROM subdivisions s WHERE s.division_id = d.id) AS subdivision_count,
       (SELECT count(*) FROM employees e JOIN subdivisions s ON s.id = e.subdivision_id
         WHERE s.division_id = d.id AND e.deleted_at IS NULL AND e.status = 'active') AS employee_count
FROM divisions d
LEFT JOIN employees h ON h.id = d.head_employee_id
ORDER BY d.name;

-- name: GetDivision :one
SELECT * FROM divisions WHERE id = $1;

-- name: CreateDivision :one
INSERT INTO divisions (code, name, head_employee_id) VALUES ($1, $2, $3) RETURNING *;

-- name: UpdateDivision :one
UPDATE divisions SET code = $2, name = $3, head_employee_id = $4 WHERE id = $1 RETURNING *;

-- name: DeleteDivision :execrows
DELETE FROM divisions WHERE id = $1;

-- name: CountSubdivisionsInDivision :one
SELECT count(*) FROM subdivisions WHERE division_id = $1;

-- name: ListSubdivisions :many
SELECT s.*, d.name AS division_name,
       h.full_name AS head_name,
       (SELECT count(*) FROM employees e
         WHERE e.subdivision_id = s.id AND e.deleted_at IS NULL AND e.status = 'active') AS employee_count
FROM subdivisions s
JOIN divisions d ON d.id = s.division_id
LEFT JOIN employees h ON h.id = s.head_employee_id
WHERE (sqlc.narg(division_id)::uuid IS NULL OR s.division_id = sqlc.narg(division_id))
ORDER BY d.name, s.name;

-- name: GetSubdivision :one
SELECT * FROM subdivisions WHERE id = $1;

-- name: CreateSubdivision :one
INSERT INTO subdivisions (division_id, code, name, head_employee_id) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: UpdateSubdivision :one
UPDATE subdivisions SET division_id = $2, code = $3, name = $4, head_employee_id = $5 WHERE id = $1 RETURNING *;

-- name: DeleteSubdivision :execrows
DELETE FROM subdivisions WHERE id = $1;

-- name: CountEmployeesInSubdivision :one
SELECT count(*) FROM employees WHERE subdivision_id = $1 AND deleted_at IS NULL;

-- name: CountEmployeesAnyStateInSubdivision :one
SELECT count(*) FROM employees WHERE subdivision_id = $1;

-- name: EmployeeBelongsToDivision :one
SELECT EXISTS (
  SELECT 1 FROM employees e JOIN subdivisions s ON s.id = e.subdivision_id
  WHERE e.id = $1 AND s.division_id = $2 AND e.deleted_at IS NULL
);

-- name: EmployeeBelongsToSubdivision :one
SELECT EXISTS (
  SELECT 1 FROM employees WHERE id = $1 AND subdivision_id = $2 AND deleted_at IS NULL
);

-- name: GetDivisionDetail :one
SELECT d.*,
       h.full_name AS head_name,
       (SELECT count(*) FROM subdivisions s WHERE s.division_id = d.id) AS subdivision_count,
       (SELECT count(*) FROM employees e JOIN subdivisions s ON s.id = e.subdivision_id
         WHERE s.division_id = d.id AND e.deleted_at IS NULL AND e.status = 'active') AS employee_count
FROM divisions d
LEFT JOIN employees h ON h.id = d.head_employee_id
WHERE d.id = $1;

-- name: GetSubdivisionDetail :one
SELECT s.*, d.name AS division_name,
       h.full_name AS head_name,
       (SELECT count(*) FROM employees e
         WHERE e.subdivision_id = s.id AND e.deleted_at IS NULL AND e.status = 'active') AS employee_count
FROM subdivisions s
JOIN divisions d ON d.id = s.division_id
LEFT JOIN employees h ON h.id = s.head_employee_id
WHERE s.id = $1;

-- name: SetDivisionHead :exec
UPDATE divisions SET head_employee_id = $2 WHERE id = $1;

-- name: SetSubdivisionHead :exec
UPDATE subdivisions SET head_employee_id = $2 WHERE id = $1;

-- name: ClearHeadOfEmployee :exec
WITH cleared AS (
  UPDATE subdivisions SET head_employee_id = NULL WHERE subdivisions.head_employee_id = @employee_id
)
UPDATE divisions SET head_employee_id = NULL WHERE divisions.head_employee_id = @employee_id;

-- Status kepala seorang Pejuang untuk menyesuaikan role otomatis (D49).
-- name: GetHeadRoleState :one
SELECT u.id AS user_id, u.role,
       EXISTS (SELECT 1 FROM divisions d WHERE d.head_employee_id = u.employee_id) AS heads_division,
       EXISTS (SELECT 1 FROM subdivisions s WHERE s.head_employee_id = u.employee_id) AS heads_subdivision
FROM users u
WHERE u.employee_id = $1;
