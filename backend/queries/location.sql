-- name: ListOfficeLocations :many
SELECT l.id, l.name, l.latitude, l.longitude, l.radius_m, l.polygon, l.is_active,
       (SELECT count(*) FROM employee_locations el WHERE el.location_id = l.id) AS employee_count
FROM office_locations l
ORDER BY l.name, l.id;

-- name: GetOfficeLocation :one
SELECT l.id, l.name, l.latitude, l.longitude, l.radius_m, l.polygon, l.is_active,
       (SELECT count(*) FROM employee_locations el WHERE el.location_id = l.id) AS employee_count
FROM office_locations l
WHERE l.id = $1;

-- name: CreateOfficeLocation :one
INSERT INTO office_locations (name, latitude, longitude, radius_m, polygon, is_active)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: UpdateOfficeLocation :exec
UPDATE office_locations
SET name = $2, latitude = $3, longitude = $4, radius_m = $5, polygon = $6, is_active = $7, updated_at = now()
WHERE id = $1;

-- name: DeleteOfficeLocation :execrows
DELETE FROM office_locations WHERE id = $1;

-- name: EmployeeExists :one
SELECT EXISTS (SELECT 1 FROM employees WHERE id = $1 AND deleted_at IS NULL);

-- name: ListEmployeeLocations :many
SELECT l.id, l.name, l.latitude, l.longitude, l.radius_m, l.polygon, l.is_active,
       (SELECT count(*) FROM employee_locations x WHERE x.location_id = l.id) AS employee_count
FROM office_locations l
JOIN employee_locations el ON el.location_id = l.id
WHERE el.employee_id = $1
ORDER BY l.name, l.id;

-- name: ClearEmployeeLocations :exec
DELETE FROM employee_locations WHERE employee_id = $1;

-- name: AddEmployeeLocation :exec
INSERT INTO employee_locations (employee_id, location_id) VALUES ($1, $2);
