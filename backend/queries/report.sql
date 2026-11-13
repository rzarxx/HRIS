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
-- missed_clock_out: presensi masuk tanpa pulang yang batasnya sudah lewat pada @now_wib (jam dinding WIB).
-- Batasnya sama dengan attendance.CloseDeadline: cutoff 04.00 esok hari, atau akhir shift malam + 4 jam.
SELECT e.id AS employee_id, e.nip, e.full_name, d.name AS division, s.name AS subdivision, e.base_salary,
       count(a.id) FILTER (WHERE a.status IN ('present','late')) AS present,
       count(a.id) FILTER (WHERE a.status = 'late') AS late,
       count(a.id) FILTER (WHERE a.status = 'leave') AS leave,
       count(a.id) FILTER (WHERE a.status = 'absent') AS absent,
       count(a.id) FILTER (WHERE a.status = 'review') AS review,
       COALESCE(sum(a.late_minutes), 0)::bigint AS late_minutes,
       COALESCE(sum(a.worked_minutes), 0)::bigint AS worked_minutes,
       count(a.id) FILTER (WHERE a.clock_in_at IS NOT NULL AND a.clock_out_at IS NULL AND a.status <> 'absent'
         AND (a.work_date + 1) + CASE WHEN sh.end_time <= sh.start_time THEN (sh.end_time - time '00:00') + interval '4 hours'
                                      ELSE interval '4 hours' END <= @now_wib::timestamp) AS missed_clock_out
FROM employees e
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
LEFT JOIN attendances a ON a.employee_id = e.id AND a.work_date >= @from_date AND a.work_date < @to_date
  AND (sqlc.arg(work_mode)::text = '' OR (a.work_mode = sqlc.arg(work_mode)::text AND a.clock_in_at IS NOT NULL))
LEFT JOIN shifts sh ON sh.id = a.shift_id
WHERE e.deleted_at IS NULL
  AND (@scope_all::boolean OR s.id = ANY(@scope_subdivision_ids::uuid[]) OR d.id = ANY(@scope_division_ids::uuid[]))
  AND (sqlc.narg(division_id)::uuid IS NULL OR d.id = sqlc.narg(division_id))
GROUP BY e.id, e.nip, e.full_name, d.name, s.name, e.base_salary, e.status, e.join_date
HAVING count(a.id) > 0 OR (sqlc.arg(work_mode)::text = '' AND e.status = 'active' AND (e.join_date IS NULL OR e.join_date < @to_date))
ORDER BY e.full_name, e.id;

-- name: AttendanceReportLeaveTypes :many
-- Hari berstatus leave per jenis cuti, dari pengajuan disetujui yang mencakup tanggalnya. Hari leave
-- tanpa pengajuan yang cocok (mis. dicatat langsung) tidak muncul di sini dan dihitung "lainnya".
SELECT a.employee_id, lt.id AS leave_type_id, lt.code, lt.name, lt.quota_kind, lt.paid, count(*) AS days
FROM attendances a
JOIN employees e ON e.id = a.employee_id
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
JOIN LATERAL (
  SELECT lr.leave_type_id FROM leave_requests lr
  WHERE lr.employee_id = a.employee_id AND lr.status = 'approved' AND a.work_date BETWEEN lr.start_date AND lr.end_date
  ORDER BY lr.created_at DESC, lr.id LIMIT 1
) m ON true
JOIN leave_types lt ON lt.id = m.leave_type_id
WHERE a.status = 'leave' AND a.work_date >= @from_date AND a.work_date < @to_date
  AND e.deleted_at IS NULL
  AND (@scope_all::boolean OR s.id = ANY(@scope_subdivision_ids::uuid[]) OR d.id = ANY(@scope_division_ids::uuid[]))
  AND (sqlc.narg(division_id)::uuid IS NULL OR d.id = sqlc.narg(division_id))
GROUP BY a.employee_id, lt.id, lt.code, lt.name, lt.quota_kind, lt.paid
ORDER BY a.employee_id, lt.name;

-- name: ListActiveEmployeesForAttendance :many
SELECT id, join_date FROM employees e WHERE deleted_at IS NULL AND status = 'active'
  AND NOT EXISTS (SELECT 1 FROM devices d WHERE d.employee_id = e.id);

-- name: GetDefaultShift :one
SELECT * FROM shifts ORDER BY name = 'Reguler' DESC, name LIMIT 1;

-- name: InsertAttendanceIfMissing :execrows
INSERT INTO attendances (employee_id, work_date, shift_id, clock_in_at, clock_out_at, status, late_minutes, risk_score)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (employee_id, work_date) DO NOTHING;

-- name: CreateShift :one
INSERT INTO shifts (name, start_time, end_time, late_tolerance_minutes) VALUES ($1, $2, $3, $4) RETURNING id;

-- name: ListAttendanceKeysSince :many
SELECT employee_id, work_date FROM attendances WHERE work_date >= $1;

-- name: AttendanceExportEmployees :many
SELECT e.id AS employee_id, e.nip, e.full_name, d.id AS division_id, d.name AS division, s.name AS subdivision,
       e.join_date, e.exit_date,
       COALESCE(sh.work_days, ARRAY[1,2,3,4,5,6]::smallint[])::smallint[] AS work_days
FROM employees e
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
LEFT JOIN shifts sh ON sh.id = e.default_shift_id
LEFT JOIN attendances a ON a.employee_id = e.id AND a.work_date >= @from_date AND a.work_date < @to_date
  AND (sqlc.arg(work_mode)::text = '' OR (a.work_mode = sqlc.arg(work_mode)::text AND a.clock_in_at IS NOT NULL))
WHERE e.deleted_at IS NULL
  AND (@scope_all::boolean OR s.id = ANY(@scope_subdivision_ids::uuid[]) OR d.id = ANY(@scope_division_ids::uuid[]))
  AND (sqlc.narg(division_id)::uuid IS NULL OR d.id = sqlc.narg(division_id))
GROUP BY e.id, e.nip, e.full_name, d.id, d.name, s.name, e.join_date, e.exit_date, e.status, sh.work_days
HAVING count(a.id) > 0 OR (sqlc.arg(work_mode)::text = '' AND e.status = 'active' AND (e.join_date IS NULL OR e.join_date < @to_date))
ORDER BY d.name, e.full_name, e.id;

-- name: AttendanceExportRecords :many
SELECT a.employee_id, a.work_date, a.clock_in_at, a.clock_out_at, a.status, a.late_minutes, a.work_mode
FROM attendances a
JOIN employees e ON e.id = a.employee_id
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
WHERE a.work_date >= @from_date AND a.work_date < @to_date
  AND (sqlc.arg(work_mode)::text = '' OR (a.work_mode = sqlc.arg(work_mode)::text AND a.clock_in_at IS NOT NULL))
  AND e.deleted_at IS NULL
  AND (@scope_all::boolean OR s.id = ANY(@scope_subdivision_ids::uuid[]) OR d.id = ANY(@scope_division_ids::uuid[]))
  AND (sqlc.narg(division_id)::uuid IS NULL OR d.id = sqlc.narg(division_id))
ORDER BY a.employee_id, a.work_date;

-- name: AttendanceExportLeaves :many
SELECT lr.employee_id, lr.start_date, lr.end_date, lt.code, lt.name, lt.quota_kind
FROM leave_requests lr
JOIN leave_types lt ON lt.id = lr.leave_type_id
JOIN employees e ON e.id = lr.employee_id
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
WHERE lr.status = 'approved'
  AND lr.start_date < @to_date AND lr.end_date >= @from_date
  AND e.deleted_at IS NULL
  AND (@scope_all::boolean OR s.id = ANY(@scope_subdivision_ids::uuid[]) OR d.id = ANY(@scope_division_ids::uuid[]))
  AND (sqlc.narg(division_id)::uuid IS NULL OR d.id = sqlc.narg(division_id))
ORDER BY lr.employee_id, lr.created_at DESC, lr.id;

-- name: AttendanceExportHolidays :many
SELECT date FROM holidays
WHERE date >= @from_date AND date < @to_date
ORDER BY date;

-- name: AttendanceExportArrangements :many
SELECT ewa.employee_id, ewa.mode, ewa.starts_on, ewa.ends_on, ewa.weekdays
FROM employee_work_arrangements ewa
JOIN employees e ON e.id = ewa.employee_id
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
WHERE ewa.starts_on < @to_date AND (ewa.ends_on IS NULL OR ewa.ends_on >= @from_date)
  AND e.deleted_at IS NULL
  AND (@scope_all::boolean OR s.id = ANY(@scope_subdivision_ids::uuid[]) OR d.id = ANY(@scope_division_ids::uuid[]))
  AND (sqlc.narg(division_id)::uuid IS NULL OR d.id = sqlc.narg(division_id))
ORDER BY ewa.employee_id, ewa.starts_on DESC, ewa.id;
