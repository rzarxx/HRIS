-- name: LockEmployeeForDevice :one
SELECT id FROM employees WHERE id = $1 AND deleted_at IS NULL FOR NO KEY UPDATE;

-- name: GetDeviceByHash :one
SELECT * FROM devices WHERE employee_id = $1 AND device_hash = $2;

-- name: CountEmployeeDevices :one
SELECT count(*) FROM devices WHERE employee_id = $1;

-- name: InsertDevice :one
INSERT INTO devices (employee_id, device_hash, model, os_version, fcm_token, status, approved_at, last_seen_at)
VALUES (@employee_id, @device_hash, @model, @os_version, @fcm_token, @status, @approved_at, now())
RETURNING *;

-- name: TouchDevice :one
UPDATE devices
SET model = @model, os_version = @os_version, fcm_token = COALESCE(@fcm_token, fcm_token), last_seen_at = now()
WHERE id = @id
RETURNING *;

-- name: ListMyDevices :many
SELECT * FROM devices WHERE employee_id = $1 ORDER BY created_at DESC, id;

-- name: ListDevices :many
SELECT d.id, d.employee_id, d.model, d.os_version, d.status, d.created_at, d.approved_at, d.revoked_at,
       d.revoked_reason, d.last_seen_at, e.full_name AS employee_name, e.nip, s.name AS subdivision_name,
       dv.name AS division_name, ae.full_name AS approved_by_name,
       EXISTS (SELECT 1 FROM users ou WHERE ou.employee_id = d.employee_id AND ou.role = 'superadmin') AS owner_is_superadmin
FROM devices d
JOIN employees e ON e.id = d.employee_id
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions dv ON dv.id = s.division_id
LEFT JOIN users au ON au.id = d.approved_by
LEFT JOIN employees ae ON ae.id = au.employee_id
WHERE sqlc.narg(status)::device_status IS NULL OR d.status = sqlc.narg(status)
ORDER BY d.created_at DESC, d.id
LIMIT @page_limit OFFSET @page_offset;

-- name: CountDevices :one
SELECT count(*) FROM devices d
WHERE sqlc.narg(status)::device_status IS NULL OR d.status = sqlc.narg(status);

-- name: GetAdminDevice :one
SELECT d.id, d.employee_id, d.model, d.os_version, d.status, d.created_at, d.approved_at, d.revoked_at,
       d.revoked_reason, d.last_seen_at, e.full_name AS employee_name, e.nip, s.name AS subdivision_name,
       dv.name AS division_name, ae.full_name AS approved_by_name,
       EXISTS (SELECT 1 FROM users ou WHERE ou.employee_id = d.employee_id AND ou.role = 'superadmin') AS owner_is_superadmin
FROM devices d
JOIN employees e ON e.id = d.employee_id
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions dv ON dv.id = s.division_id
LEFT JOIN users au ON au.id = d.approved_by
LEFT JOIN employees ae ON ae.id = au.employee_id
WHERE d.id = $1;

-- name: GetDeviceForUpdate :one
SELECT * FROM devices WHERE id = $1 FOR UPDATE;

-- name: ReplaceApprovedDevices :many
UPDATE devices
SET status = 'revoked', revoked_by = @actor, revoked_at = now(), revoked_reason = 'replaced'
WHERE employee_id = @employee_id AND status = 'approved' AND id <> @keep_id
RETURNING id;

-- name: ApproveDevice :exec
UPDATE devices SET status = 'approved', approved_by = @actor, approved_at = now() WHERE id = @id;

-- name: RevokeDevice :exec
UPDATE devices
SET status = 'revoked', revoked_by = @actor, revoked_at = now(), revoked_reason = @reason
WHERE id = @id;

-- name: RevokeEmployeeDevices :exec
UPDATE devices
SET status = 'revoked', revoked_at = now(), revoked_reason = 'offboarded'
WHERE employee_id = $1 AND status <> 'revoked';

-- name: RevokeDeviceSessions :exec
UPDATE sessions SET revoked_at = now(), revoke_reason = @reason
WHERE device_id = @device_id AND revoked_at IS NULL;

-- name: GetEmployeeStatus :one
SELECT status FROM employees WHERE id = $1 AND deleted_at IS NULL;

-- name: SetUserRole :exec
UPDATE users SET role = @role WHERE id = @id;
