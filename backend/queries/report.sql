-- name: DashboardSummary :one
SELECT
  (SELECT count(*) FROM employees e WHERE e.deleted_at IS NULL AND e.status = 'active') AS employee_count,
  (SELECT count(*) FROM divisions) AS division_count,
  (SELECT count(*) FROM subdivisions) AS subdivision_count,
  (SELECT count(*) FROM attendances a JOIN employees e ON e.id = a.employee_id
    WHERE a.work_date = @work_date AND e.deleted_at IS NULL AND e.status = 'active' AND a.status IN ('present','late')) AS present_today,
  (SELECT count(*) FROM attendances a JOIN employees e ON e.id = a.employee_id
    WHERE a.work_date = @work_date AND e.deleted_at IS NULL AND e.status = 'active' AND a.status = 'late') AS late_today;

-- name: EmployeesPerSubdivision :many
SELECT d.name AS division, s.name AS subdivision, count(e.id) AS employee_count
FROM subdivisions s
JOIN divisions d ON d.id = s.division_id
LEFT JOIN employees e ON e.subdivision_id = s.id AND e.deleted_at IS NULL AND e.status = 'active'
GROUP BY d.name, s.name
ORDER BY d.name, s.name;

-- name: SubdivisionsPerDivision :many
SELECT d.name AS division, count(s.id) AS subdivision_count
FROM divisions d
LEFT JOIN subdivisions s ON s.division_id = d.id
GROUP BY d.name
ORDER BY d.name;

-- name: AttendanceReport :many
-- Masuk periode bila punya presensi, atau aktif dan sudah bergabung sebelum akhir bulan.
SELECT e.id AS employee_id, e.nip, e.full_name, d.name AS division, s.name AS subdivision, e.base_salary,
       count(a.id) FILTER (WHERE a.status IN ('present','late')) AS present,
       count(a.id) FILTER (WHERE a.status = 'late') AS late,
       count(a.id) FILTER (WHERE a.status = 'leave') AS leave,
       count(a.id) FILTER (WHERE a.status = 'absent') AS absent,
       COALESCE(sum(a.late_minutes), 0)::bigint AS late_minutes
FROM employees e
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
LEFT JOIN attendances a ON a.employee_id = e.id AND a.work_date >= @from_date AND a.work_date < @to_date
WHERE e.deleted_at IS NULL
  AND (@scope_all::boolean OR s.id = ANY(@scope_subdivision_ids::uuid[]) OR d.id = ANY(@scope_division_ids::uuid[]))
  AND (sqlc.narg(division_id)::uuid IS NULL OR d.id = sqlc.narg(division_id))
GROUP BY e.id, e.nip, e.full_name, d.name, s.name, e.base_salary, e.status, e.join_date
HAVING count(a.id) > 0 OR (e.status = 'active' AND (e.join_date IS NULL OR e.join_date < @to_date))
ORDER BY e.full_name, e.id;

-- name: ListActiveEmployeesForAttendance :many
SELECT id, join_date FROM employees WHERE deleted_at IS NULL AND status = 'active';

-- name: GetDefaultShift :one
SELECT * FROM shifts ORDER BY name LIMIT 1;

-- name: InsertAttendanceIfMissing :execrows
INSERT INTO attendances (employee_id, work_date, shift_id, clock_in_at, clock_out_at, status, late_minutes, risk_score)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (employee_id, work_date) DO NOTHING;

-- name: CreateShift :one
INSERT INTO shifts (name, start_time, end_time, late_tolerance_minutes) VALUES ($1, $2, $3, $4) RETURNING id;

-- name: ListAttendanceKeysSince :many
SELECT employee_id, work_date FROM attendances WHERE work_date >= $1;
