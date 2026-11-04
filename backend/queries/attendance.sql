-- name: GetOwnDevice :one
SELECT * FROM devices WHERE id = $1 AND employee_id = $2;

-- name: CreateAttendanceChallenge :one
INSERT INTO attendance_challenges (employee_id, device_id, nonce, purpose, expires_at)
VALUES (@employee_id, @device_id, @nonce, @purpose, @expires_at)
RETURNING *;

-- Pemakaian atomik: hanya satu permintaan yang bisa memakai satu tantangan.
-- name: ConsumeAttendanceChallenge :one
UPDATE attendance_challenges SET used_at = @now
WHERE id = @id AND employee_id = @employee_id AND device_id = @device_id AND purpose = @purpose
  AND used_at IS NULL AND expires_at > @now
RETURNING id;

-- name: InsertAttendanceAttempt :exec
INSERT INTO attendance_attempts (employee_id, device_id, challenge_id, purpose, outcome, reason_code, risk_flags, latitude, longitude, ip)
VALUES (@employee_id, @device_id, @challenge_id, @purpose, @outcome, @reason_code, @risk_flags, @latitude, @longitude, @ip);

-- name: GetEmployeeDefaultShift :one
SELECT sh.* FROM employees e JOIN shifts sh ON sh.id = e.default_shift_id WHERE e.id = $1;

-- name: IsHoliday :one
SELECT EXISTS (SELECT 1 FROM holidays WHERE date = $1);

-- name: ClockIn :one
INSERT INTO attendances (employee_id, work_date, shift_id, clock_in_at, in_accuracy_m, risk_flags, status, late_minutes, work_mode, flexible_hours, location_id, in_latitude, in_longitude)
VALUES (@employee_id, @work_date, @shift_id, @clock_in_at, @in_accuracy_m, @risk_flags, @status, @late_minutes, @work_mode, @flexible_hours, @location_id, @in_latitude, @in_longitude)
ON CONFLICT (employee_id, work_date) DO NOTHING
RETURNING *;

-- name: GetOpenAttendance :one
SELECT * FROM attendances
WHERE employee_id = $1 AND clock_in_at IS NOT NULL AND clock_out_at IS NULL
ORDER BY work_date DESC LIMIT 1;

-- name: GetLastClosedAttendance :one
SELECT * FROM attendances
WHERE employee_id = $1 AND clock_out_at IS NOT NULL
ORDER BY work_date DESC LIMIT 1;

-- name: ClockOut :one
UPDATE attendances
SET clock_out_at = @clock_out_at, worked_minutes = @worked_minutes, out_latitude = @out_latitude, out_longitude = @out_longitude, work_mode = @work_mode
WHERE id = @id AND clock_out_at IS NULL
RETURNING *;

-- name: ListMyAttendance :many
SELECT a.id, a.work_date, a.status, a.clock_in_at, a.clock_out_at, a.late_minutes, a.worked_minutes, a.work_mode, l.name AS location_name
FROM attendances a
LEFT JOIN office_locations l ON l.id = a.location_id
WHERE a.employee_id = @employee_id AND a.work_date >= @from_date AND a.work_date <= @to_date
ORDER BY a.work_date DESC;

-- name: ListWorkArrangements :many
SELECT * FROM employee_work_arrangements WHERE employee_id = $1 ORDER BY starts_on DESC, id;

-- name: InsertWorkArrangement :one
INSERT INTO employee_work_arrangements (employee_id, mode, flexible_hours, starts_on, ends_on, weekdays, reason, created_by)
VALUES (@employee_id, @mode, @flexible_hours, @starts_on, @ends_on, @weekdays, @reason, @created_by)
RETURNING *;

-- name: DeleteWorkArrangement :one
DELETE FROM employee_work_arrangements WHERE id = @id AND employee_id = @employee_id RETURNING *;
