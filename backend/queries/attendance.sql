-- name: GetOwnDevice :one
SELECT * FROM devices WHERE id = $1 AND employee_id = $2;

-- name: CreateAttendanceChallenge :one
INSERT INTO attendance_challenges (employee_id, device_id, nonce, purpose, expires_at, liveness_action)
VALUES (@employee_id, @device_id, @nonce, @purpose, @expires_at, @liveness_action)
RETURNING *;

-- Pemakaian atomik: hanya satu permintaan yang bisa memakai satu tantangan.
-- name: ConsumeAttendanceChallenge :one
UPDATE attendance_challenges SET used_at = @now
WHERE id = @id AND employee_id = @employee_id AND device_id = @device_id AND purpose = @purpose
  AND used_at IS NULL AND expires_at > @now
RETURNING id, nonce, liveness_action;

-- name: InsertAttendanceAttempt :exec
INSERT INTO attendance_attempts (employee_id, device_id, challenge_id, purpose, outcome, reason_code, risk_score, risk_flags, latitude, longitude, ip)
VALUES (@employee_id, @device_id, @challenge_id, @purpose, @outcome, @reason_code, @risk_score, @risk_flags, @latitude, @longitude, @ip);

-- name: GetEmployeeDefaultShift :one
SELECT sh.* FROM employees e JOIN shifts sh ON sh.id = e.default_shift_id WHERE e.id = $1;

-- name: IsHoliday :one
SELECT EXISTS (SELECT 1 FROM holidays WHERE date = $1);

-- name: ClockIn :one
INSERT INTO attendances (employee_id, work_date, shift_id, clock_in_at, in_accuracy_m, risk_score, risk_flags, status, late_minutes, work_mode, flexible_hours, location_id, in_latitude, in_longitude, face_score)
VALUES (@employee_id, @work_date, @shift_id, @clock_in_at, @in_accuracy_m, @risk_score, @risk_flags, @status, @late_minutes, @work_mode, @flexible_hours, @location_id, @in_latitude, @in_longitude, @face_score)
ON CONFLICT (employee_id, work_date) DO NOTHING
RETURNING *;

-- name: GetOpenAttendance :one
SELECT * FROM attendances
WHERE employee_id = $1 AND clock_in_at IS NOT NULL AND clock_out_at IS NULL AND status <> 'absent'
ORDER BY work_date DESC LIMIT 1;

-- name: GetLastClosedAttendance :one
SELECT * FROM attendances
WHERE employee_id = $1 AND clock_out_at IS NOT NULL
ORDER BY work_date DESC LIMIT 1;

-- name: ClockOut :one
UPDATE attendances
SET clock_out_at = @clock_out_at, worked_minutes = @worked_minutes, out_latitude = @out_latitude, out_longitude = @out_longitude, work_mode = @work_mode,
    status = @status, risk_score = @risk_score, risk_flags = @risk_flags
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

-- name: GetLastAttendancePoint :one
SELECT clock_in_at, clock_out_at, in_latitude, in_longitude, out_latitude, out_longitude FROM attendances
WHERE employee_id = $1 AND in_latitude IS NOT NULL
ORDER BY work_date DESC LIMIT 1;

-- name: ListAttendanceReview :many
SELECT a.id, a.employee_id, a.work_date, a.clock_in_at, a.clock_out_at, a.late_minutes, a.work_mode, a.flexible_hours,
       a.in_latitude, a.in_longitude, a.out_latitude, a.out_longitude, a.in_accuracy_m, a.risk_score, a.risk_flags,
       e.full_name AS employee_name, e.nip, s.name AS subdivision_name, d.name AS division_name, l.name AS location_name
FROM attendances a
JOIN employees e ON e.id = a.employee_id
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
LEFT JOIN office_locations l ON l.id = a.location_id
WHERE a.status = 'review' AND (sqlc.narg(work_mode)::text IS NULL OR a.work_mode = sqlc.narg(work_mode))
ORDER BY a.work_date, a.clock_in_at, a.id
LIMIT @page_limit OFFSET @page_offset;

-- name: CountAttendanceReview :one
SELECT count(*) FROM attendances a
WHERE a.status = 'review' AND (sqlc.narg(work_mode)::text IS NULL OR a.work_mode = sqlc.narg(work_mode));

-- name: GetAttendanceForUpdate :one
SELECT * FROM attendances WHERE id = $1 FOR UPDATE;

-- name: ResolveAttendanceReview :one
UPDATE attendances
SET status = @status, late_minutes = @late_minutes, review_by = @review_by, reviewed_at = @reviewed_at, review_note = @review_note
WHERE id = @id
RETURNING *;

-- name: ListAttendanceMonitor :many
SELECT e.id, e.nip, e.full_name, s.name AS subdivision_name, d.id AS division_id, d.name AS division_name,
       sh.name AS shift_name, sh.work_days, a.status, a.clock_in_at, a.clock_out_at, a.late_minutes, a.work_mode,
       l.name AS location_name, a.in_latitude, a.in_longitude, a.out_latitude, a.out_longitude
FROM employees e
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
LEFT JOIN shifts sh ON sh.id = e.default_shift_id
LEFT JOIN attendances a ON a.employee_id = e.id AND a.work_date = @work_date
LEFT JOIN office_locations l ON l.id = a.location_id
WHERE e.deleted_at IS NULL AND e.status = 'active'
  AND (e.join_date IS NULL OR e.join_date <= @work_date)
  AND (sqlc.narg(division_id)::uuid IS NULL OR d.id = sqlc.narg(division_id))
ORDER BY d.name, e.full_name;

-- name: ListArrangementTargets :many
SELECT e.id, e.full_name
FROM employees e JOIN subdivisions s ON s.id = e.subdivision_id
WHERE e.deleted_at IS NULL AND e.status = 'active'
  AND (sqlc.narg(division_id)::uuid IS NULL OR s.division_id = sqlc.narg(division_id))
  AND (sqlc.narg(subdivision_id)::uuid IS NULL OR s.id = sqlc.narg(subdivision_id))
ORDER BY e.id;

-- name: CountAttendanceAttempts :one
SELECT count(*) FROM attendance_attempts
WHERE created_at >= @from_at AND created_at < @to_at
  AND (sqlc.narg(outcome)::text IS NULL OR outcome = sqlc.narg(outcome));

-- name: ListAttendanceAttempts :many
SELECT a.id, a.employee_id, e.full_name AS employee_name, e.nip, s.name AS subdivision_name, d.name AS division_name,
       a.purpose, a.outcome, a.reason_code, a.risk_score, a.risk_flags, a.latitude, a.longitude, a.created_at
FROM attendance_attempts a
JOIN employees e ON e.id = a.employee_id
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
WHERE a.created_at >= @from_at AND a.created_at < @to_at
  AND (sqlc.narg(outcome)::text IS NULL OR a.outcome = sqlc.narg(outcome))
ORDER BY a.created_at DESC, a.id DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: GetActiveFaceTemplate :one
SELECT * FROM face_templates WHERE employee_id = $1 AND is_active;

-- name: DeactivateFaceTemplates :execrows
UPDATE face_templates SET is_active = false, deactivated_at = now(), deactivated_by = @actor
WHERE employee_id = @employee_id AND is_active;

-- name: InsertFaceTemplate :one
INSERT INTO face_templates (employee_id, device_id, embedding_enc, key_id, model_version, dimension)
VALUES (@employee_id, @device_id, @embedding_enc, @key_id, @model_version, @dimension)
RETURNING *;

-- name: InsertConsent :exec
INSERT INTO consents (employee_id, type, version, ip) VALUES (@employee_id, @type, @version, @ip);

-- Selfie yang sama tidak boleh dipakai dua kali oleh Pejuang yang sama (embedding_replay).
-- name: InsertFaceCapture :one
INSERT INTO face_captures (employee_id, challenge_id, purpose, sha256, storage_key)
VALUES (@employee_id, @challenge_id, @purpose, @sha256, @storage_key)
ON CONFLICT (employee_id, sha256) DO NOTHING
RETURNING id;

-- name: SetFaceCaptureScore :exec
UPDATE face_captures SET face_score = @face_score WHERE id = @id;

-- name: LinkFaceCapture :exec
UPDATE face_captures SET attendance_id = @attendance_id WHERE id = @id;

-- Satu baris dikunci selama berkas dihapus. SKIP LOCKED memungkinkan beberapa pekerja.
-- name: LockExpiredFaceCapture :one
SELECT id, storage_key FROM face_captures
WHERE selfie_deleted_at IS NULL AND created_at < @cutoff
ORDER BY created_at, id LIMIT 1 FOR UPDATE SKIP LOCKED;

-- name: MarkFaceCaptureDeleted :exec
UPDATE face_captures SET selfie_deleted_at = now() WHERE id = @id;

-- name: LockReviewSelfie :one
SELECT f.id, f.storage_key FROM face_captures f
JOIN attendances a ON a.id = f.attendance_id
WHERE a.id = @attendance_id AND a.status = 'review' AND f.purpose = @purpose
  AND f.selfie_deleted_at IS NULL AND f.created_at > @cutoff
ORDER BY f.created_at DESC LIMIT 1 FOR UPDATE OF a, f;

-- name: GetRemoteFacePermission :one
SELECT * FROM face_remote_permissions WHERE employee_id = @employee_id AND expires_at > @now;

-- name: SetRemoteFacePermission :exec
INSERT INTO face_remote_permissions (employee_id, approved_by, reason, expires_at)
VALUES (@employee_id, @approved_by, @reason, @expires_at)
ON CONFLICT (employee_id) DO UPDATE SET id = EXCLUDED.id, approved_by = EXCLUDED.approved_by,
 reason = EXCLUDED.reason, expires_at = EXCLUDED.expires_at, created_at = now();

-- name: DeleteRemoteFacePermission :execrows
DELETE FROM face_remote_permissions WHERE employee_id = @employee_id;

-- name: ConsumeRemoteFacePermission :execrows
DELETE FROM face_remote_permissions WHERE employee_id = @employee_id AND id = @id AND expires_at > @now;
