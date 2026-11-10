-- name: ListLeaveTypes :many
SELECT * FROM leave_types ORDER BY is_active DESC, name;

-- name: GetLeaveType :one
SELECT * FROM leave_types WHERE id = $1;

-- name: InsertLeaveType :one
INSERT INTO leave_types (code, name, paid, quota_kind, annual_quota_days, is_active)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateLeaveType :one
UPDATE leave_types
SET name = $2, paid = $3, quota_kind = $4, annual_quota_days = $5, is_active = $6
WHERE id = $1
RETURNING *;

-- name: GetLeaveProfile :one
SELECT e.id, e.join_date, sh.start_time, sh.end_time, sh.late_tolerance_minutes, sh.work_days
FROM employees e
LEFT JOIN shifts sh ON sh.id = e.default_shift_id
WHERE e.id = $1 AND e.deleted_at IS NULL;

-- Hari terpakai per jenis dalam satu tahun kalender: pengajuan pending ikut dihitung agar saldo tidak
-- bisa dipakai dua kali sebelum keputusan keluar.
-- name: UsedLeaveDays :many
SELECT leave_type_id, COALESCE(SUM(days), 0)::int AS days
FROM leave_requests
WHERE employee_id = $1 AND status IN ('pending', 'approved')
  AND start_date >= @year_start::date AND start_date < @year_end::date
GROUP BY leave_type_id;

-- name: CountOverlappingLeaves :one
SELECT count(*) FROM leave_requests
WHERE employee_id = $1 AND status IN ('pending', 'approved')
  AND start_date <= @end_date::date AND end_date >= @start_date::date;

-- name: AttendedDatesInRange :many
SELECT work_date FROM attendances
WHERE employee_id = $1 AND clock_in_at IS NOT NULL
  AND work_date BETWEEN @start_date::date AND @end_date::date
ORDER BY work_date;

-- name: HolidaysInRange :many
SELECT date FROM holidays WHERE date BETWEEN @start_date::date AND @end_date::date;

-- name: InsertLeaveRequest :one
INSERT INTO leave_requests (employee_id, leave_type_id, start_date, end_date, days, reason)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListMyLeaves :many
SELECT lr.*, lt.name AS leave_type_name, lt.code AS leave_type_code
FROM leave_requests lr
JOIN leave_types lt ON lt.id = lr.leave_type_id
WHERE lr.employee_id = $1
ORDER BY lr.start_date DESC, lr.created_at DESC
LIMIT 100;

-- name: GetLeaveDetail :one
SELECT lr.*, lt.name AS leave_type_name, lt.code AS leave_type_code,
       e.full_name AS employee_name, s.name AS subdivision_name
FROM leave_requests lr
JOIN leave_types lt ON lt.id = lr.leave_type_id
JOIN employees e ON e.id = lr.employee_id
JOIN subdivisions s ON s.id = e.subdivision_id
WHERE lr.id = $1;

-- name: LockLeaveRequest :one
SELECT * FROM leave_requests WHERE id = $1 FOR UPDATE;

-- name: SetLeaveStatus :exec
UPDATE leave_requests SET status = @status, decided_at = @decided_at WHERE id = @id;

-- Hari cuti yang disetujui tercatat di presensi. Hari yang sudah ada absen masuk tidak ditimpa.
-- name: MarkLeaveAttendance :exec
INSERT INTO attendances (employee_id, work_date, status)
VALUES ($1, $2, 'leave')
ON CONFLICT (employee_id, work_date) DO UPDATE SET status = 'leave'
WHERE attendances.clock_in_at IS NULL;

-- Filter scope sama dengan modul pegawai: scope_all untuk SDM, selain itu subdiv/divisi yang dipimpin.
-- name: ListLeavesAdmin :many
SELECT lr.*, lt.name AS leave_type_name, lt.code AS leave_type_code,
       e.full_name AS employee_name, s.name AS subdivision_name
FROM leave_requests lr
JOIN leave_types lt ON lt.id = lr.leave_type_id
JOIN employees e ON e.id = lr.employee_id
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
WHERE (@scope_all::boolean OR s.id = ANY(@scope_subdivision_ids::uuid[]) OR d.id = ANY(@scope_division_ids::uuid[]))
  AND (sqlc.narg(status)::request_status IS NULL OR lr.status = sqlc.narg(status)::request_status)
  AND lr.end_date >= @from_date::date AND lr.start_date <= @to_date::date
ORDER BY lr.start_date DESC, lr.created_at DESC
LIMIT 500;

-- name: LeaveSummaries :many
SELECT lr.id, lr.start_date, lr.end_date, lr.days, lr.reason, lt.name AS leave_type_name
FROM leave_requests lr
JOIN leave_types lt ON lt.id = lr.leave_type_id
WHERE lr.id = ANY(@ids::uuid[]);
