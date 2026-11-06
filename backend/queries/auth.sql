-- name: GetUserByEmail :one
SELECT u.*, e.full_name
FROM users u JOIN employees e ON e.id = u.employee_id
WHERE lower(u.email) = lower($1) AND e.deleted_at IS NULL;

-- name: RegisterLoginFailure :one
UPDATE users
SET failed_login_count = failed_login_count + 1,
    locked_until = CASE WHEN failed_login_count + 1 >= @max_attempts::int THEN @locked_until::timestamptz ELSE locked_until END
WHERE id = @id
RETURNING failed_login_count, locked_until;

-- name: RegisterLoginSuccess :exec
UPDATE users SET failed_login_count = 0, locked_until = NULL, last_login_at = now() WHERE id = $1;

-- name: CreateSession :one
INSERT INTO sessions (user_id, client, device_id, refresh_hash, ip, user_agent, idle_expires_at, absolute_expires_at)
VALUES (@user_id, @client, sqlc.narg(device_id), @refresh_hash, @ip, @user_agent, @idle_expires_at, @absolute_expires_at)
RETURNING id;

-- name: GetSessionByRefreshHash :one
SELECT s.*, u.employee_id, u.role, u.is_active, d.status AS device_status
FROM sessions s JOIN users u ON u.id = s.user_id
LEFT JOIN devices d ON d.id = s.device_id AND d.employee_id = u.employee_id
WHERE s.refresh_hash = $1;

-- name: GetSessionByPrevRefreshHash :one
SELECT id, user_id FROM sessions WHERE prev_refresh_hash = $1 AND client = @client AND revoked_at IS NULL;

-- name: RotateSession :exec
UPDATE sessions
SET prev_refresh_hash = refresh_hash, refresh_hash = @new_hash, last_used_at = now(), idle_expires_at = @idle_expires_at
WHERE id = @id;

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at = now(), revoke_reason = $2 WHERE id = $1 AND revoked_at IS NULL;

-- name: GetMe :one
SELECT u.id AS user_id, u.employee_id, u.role, u.email, u.must_change_password,
       e.full_name, e.nip, e.position,
       s.id AS subdivision_id, s.name AS subdivision_name,
       d.id AS division_id, d.name AS division_name
FROM users u
JOIN employees e ON e.id = u.employee_id
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
WHERE u.id = $1;

-- name: InsertAuditLog :exec
INSERT INTO audit_logs (user_id, action, entity, entity_id, detail, ip)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: HeadedSubdivisionIDs :many
SELECT id FROM subdivisions WHERE head_employee_id = $1;

-- name: HeadedDivisionIDs :many
SELECT id FROM divisions WHERE head_employee_id = $1;

-- name: CreateUser :one
INSERT INTO users (employee_id, email, password_hash, role, must_change_password, password_changed_at)
VALUES ($1, $2, $3, $4, $5, now())
RETURNING id;

-- name: GetSessionAuthState :one
-- Status sesi + syarat yang harus dipenuhi sebelum memakai fitur lain.
SELECT (s.revoked_at IS NULL AND s.absolute_expires_at > now() AND u.is_active)::boolean AS is_active,
       u.must_change_password,
       u.role,
       s.user_id, u.employee_id, s.client, s.device_id,
       (d.status IS NOT NULL AND d.status <> 'revoked')::boolean AS device_active,
       (m.enabled_at IS NOT NULL)::boolean AS mfa_enabled
FROM sessions s
JOIN users u ON u.id = s.user_id
LEFT JOIN user_mfa m ON m.user_id = u.id
LEFT JOIN devices d ON d.id = s.device_id AND d.employee_id = u.employee_id
WHERE s.id = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: SetUserPassword :exec
UPDATE users
SET password_hash = @password_hash, must_change_password = false, password_changed_at = now()
WHERE id = @id;

-- name: RevokeOtherSessions :exec
UPDATE sessions SET revoked_at = now(), revoke_reason = @reason
WHERE user_id = @user_id AND id <> @keep_session_id AND revoked_at IS NULL;

-- name: RevokeAllSessions :exec
UPDATE sessions SET revoked_at = now(), revoke_reason = @reason
WHERE user_id = @user_id AND revoked_at IS NULL;

-- name: ListActiveSessions :many
SELECT id, client, ip, user_agent, created_at, last_used_at
FROM sessions
WHERE user_id = $1 AND revoked_at IS NULL AND absolute_expires_at > now()
ORDER BY last_used_at DESC;

-- name: RevokeOwnSession :execrows
UPDATE sessions SET revoked_at = now(), revoke_reason = 'user_revoked'
WHERE id = @id AND user_id = @user_id AND revoked_at IS NULL;

-- name: GetUserMfa :one
SELECT * FROM user_mfa WHERE user_id = $1;

-- name: UpsertPendingMfa :exec
INSERT INTO user_mfa (user_id, secret_enc) VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET secret_enc = EXCLUDED.secret_enc, enabled_at = NULL, recovery_hashes = '{}', last_used_step = 0
WHERE user_mfa.enabled_at IS NULL;

-- name: EnableMfa :exec
UPDATE user_mfa SET enabled_at = now(), recovery_hashes = @recovery_hashes, last_used_step = @last_used_step
WHERE user_id = @user_id;

-- name: SetMfaLastStep :execrows
-- Bersyarat: kode yang sama tidak bisa dipakai dua kali walau permintaan datang bersamaan.
UPDATE user_mfa SET last_used_step = @step WHERE user_id = @user_id AND last_used_step < @step;

-- name: SetMfaRecoveryHashes :exec
UPDATE user_mfa SET recovery_hashes = @recovery_hashes WHERE user_id = @user_id;

-- name: CreateUserInvite :exec
INSERT INTO user_invites (user_id, token_hash, expires_at) VALUES ($1, $2, $3);

-- name: InvalidateUserInvites :exec
UPDATE user_invites SET used_at = now() WHERE user_id = $1 AND used_at IS NULL;

-- name: ConsumeInvite :one
-- Sekali pakai: UPDATE bersyarat agar dua permintaan bersamaan tidak sama-sama lolos.
UPDATE user_invites SET used_at = now()
WHERE token_hash = @token_hash AND used_at IS NULL AND expires_at > now()
RETURNING user_id;

-- name: GetInviteState :one
SELECT (used_at IS NOT NULL)::bool AS used, (expires_at <= now())::bool AS expired, expires_at
FROM user_invites WHERE token_hash = @token_hash;

-- name: ActivateUser :exec
UPDATE users SET is_active = true WHERE id = $1;

-- name: SetUserLock :exec
UPDATE users SET locked_until = @locked_until, failed_login_count = 0 WHERE id = @id;

-- name: SetUserActive :exec
UPDATE users SET is_active = @is_active WHERE id = @id;

-- name: GetUserByEmployee :one
SELECT * FROM users WHERE employee_id = $1;

-- name: CreateInactiveUser :one
INSERT INTO users (employee_id, email, password_hash, role, is_active, must_change_password)
VALUES ($1, $2, $3, $4, false, false)
RETURNING id;

-- name: GetEmployeeAccount :one
SELECT u.id AS user_id, u.role, u.is_active, u.locked_until,
       (m.enabled_at IS NOT NULL)::boolean AS mfa_enabled
FROM users u
LEFT JOIN user_mfa m ON m.user_id = u.id
WHERE u.employee_id = $1;

-- name: ListEmployeeAccounts :many
SELECT u.employee_id, u.id AS user_id, u.role, u.is_active, u.locked_until,
       (m.enabled_at IS NOT NULL)::boolean AS mfa_enabled
FROM users u
LEFT JOIN user_mfa m ON m.user_id = u.id
WHERE u.employee_id = ANY(@employee_ids::uuid[]);

-- name: UpdateUserEmail :exec
UPDATE users SET email = @email WHERE employee_id = @employee_id;

-- name: GetDeviceStatus :one
SELECT status FROM devices WHERE id = $1;
