-- name: ListShifts :many
SELECT * FROM shifts ORDER BY start_time, name;

-- name: GetShift :one
SELECT * FROM shifts WHERE id = $1;

-- name: InsertShift :one
INSERT INTO shifts (name, start_time, end_time, late_tolerance_minutes, work_days)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateShift :one
UPDATE shifts SET name = $2, start_time = $3, end_time = $4, late_tolerance_minutes = $5, work_days = $6
WHERE id = $1
RETURNING *;

-- name: DeleteShift :execrows
DELETE FROM shifts WHERE id = $1;

-- name: ListHolidays :many
SELECT * FROM holidays WHERE date >= @from_date AND date < @to_date ORDER BY date;

-- name: InsertHoliday :exec
INSERT INTO holidays (date, name) VALUES ($1, $2);

-- name: DeleteHoliday :execrows
DELETE FROM holidays WHERE date = $1;
