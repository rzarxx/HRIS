-- Login memakai NIPY, email, atau nomor WA (D53). Pemanggil mengisi tepat satu parameter.
-- name: GetUserForLogin :one
SELECT u.*, e.full_name
FROM users u JOIN employees e ON e.id = u.employee_id
WHERE e.deleted_at IS NULL
  AND ((sqlc.narg(email)::text IS NOT NULL AND lower(u.email) = lower(sqlc.narg(email)))
    OR (sqlc.narg(nip)::text IS NOT NULL AND upper(e.nip) = upper(sqlc.narg(nip)))
    OR (sqlc.narg(phone)::text IS NOT NULL AND e.phone = sqlc.narg(phone)));

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
SELECT u.id AS user_id, u.employee_id, u.role, u.email, u.must_change_password, u.mfa_required,
       e.full_name, e.nip, e.position, e.phone, e.photo_updated_at, (e.photo_key IS NOT NULL)::boolean AS has_photo,
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
       u.mfa_required,
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
SET password_hash = @password_hash, must_change_password = false, password_changed_at = now(),
    initial_password_expires_at = NULL
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

-- name: InvalidateUserInvites :exec
UPDATE user_invites SET used_at = now() WHERE user_id = $1 AND used_at IS NULL;

-- name: SetUserLock :exec
UPDATE users SET locked_until = @locked_until, failed_login_count = 0 WHERE id = @id;

-- name: SetUserActive :exec
UPDATE users SET is_active = @is_active WHERE id = @id;

-- name: GetUserByEmployee :one
SELECT * FROM users WHERE employee_id = $1;

-- Akun baru langsung aktif dengan kata sandi awal yang wajib diganti saat login pertama (D53).
-- name: CreateUserWithInitialPassword :one
INSERT INTO users (employee_id, email, password_hash, role, is_active, must_change_password, initial_password_expires_at)
VALUES (@employee_id, @email, @password_hash, @role, true, true, @initial_password_expires_at)
RETURNING id;

-- name: SetInitialPassword :exec
UPDATE users
SET password_hash = @password_hash, must_change_password = true, password_changed_at = now(),
    initial_password_expires_at = @initial_password_expires_at, failed_login_count = 0, locked_until = NULL
WHERE id = @id;

-- name: GetEmployeeAccount :one
SELECT u.id AS user_id, u.role, u.is_active, u.locked_until,
       (m.enabled_at IS NOT NULL)::boolean AS mfa_enabled
FROM users u
LEFT JOIN user_mfa m ON m.user_id = u.id
WHERE u.employee_id = $1;

-- name: ListEmployeeAccounts :many
SELECT u.employee_id, u.id AS user_id, u.role, u.is_active, u.locked_until, u.mfa_required,
       (m.enabled_at IS NOT NULL)::boolean AS mfa_enabled
FROM users u
LEFT JOIN user_mfa m ON m.user_id = u.id
WHERE u.employee_id = ANY(@employee_ids::uuid[]);

-- name: UpdateUserEmail :exec
UPDATE users SET email = sqlc.narg(email) WHERE employee_id = @employee_id;

-- name: GetDeviceStatus :one
SELECT status FROM devices WHERE id = $1;

-- name: EmployeeHasFace :one
SELECT EXISTS (SELECT 1 FROM face_templates WHERE employee_id = $1 AND is_active);

-- name: HasSDMUser :one
SELECT EXISTS (SELECT 1 FROM users WHERE role = 'sdm');

-- Tautan baru membatalkan tautan reset lama milik akun yang sama.
-- name: InvalidatePasswordResetTokens :exec
UPDATE password_reset_tokens SET used_at = @now WHERE user_id = @user_id AND used_at IS NULL;

-- name: CreatePasswordResetToken :exec
INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES (@user_id, @token_hash, @expires_at);

-- Pemakaian atomik: hanya satu permintaan yang bisa memakai satu tautan.
-- name: ConsumePasswordResetToken :one
UPDATE password_reset_tokens SET used_at = @now
WHERE token_hash = @token_hash AND used_at IS NULL AND expires_at > @now
RETURNING user_id;

-- Ganti email lewat konfirmasi (D59). Permintaan baru membatalkan permintaan lama.
-- name: InvalidateEmailChangeRequests :exec
UPDATE email_change_requests SET used_at = @now WHERE user_id = @user_id AND used_at IS NULL;

-- name: CountEmailChangeRequestsSince :one
SELECT count(*) FROM email_change_requests WHERE user_id = @user_id AND created_at > @since;

-- name: CreateEmailChangeRequest :exec
INSERT INTO email_change_requests (user_id, new_email, token_hash, expires_at)
VALUES (@user_id, @new_email, @token_hash, @expires_at);

-- name: GetPendingEmailChange :one
SELECT new_email FROM email_change_requests
WHERE user_id = @user_id AND used_at IS NULL AND expires_at > @now
ORDER BY created_at DESC LIMIT 1;

-- Pemakaian atomik: satu tautan hanya bisa dipakai sekali.
-- name: ConsumeEmailChangeRequest :one
UPDATE email_change_requests SET used_at = @now
WHERE token_hash = @token_hash AND used_at IS NULL AND expires_at > @now
RETURNING user_id, new_email;

-- name: EmailInUse :one
SELECT EXISTS (
  SELECT 1 FROM employees e WHERE lower(e.email) = lower(@email::text) AND e.id <> @employee_id::uuid
  UNION ALL
  SELECT 1 FROM users u WHERE lower(u.email) = lower(@email::text) AND u.employee_id <> @employee_id::uuid
);

-- name: SetEmployeeEmail :execrows
UPDATE employees SET email = sqlc.narg(email), updated_at = now() WHERE id = @id;

-- A2F wajib per akun SDM (D60), diatur super admin.
-- name: SetUserMFARequired :exec
UPDATE users SET mfa_required = @mfa_required WHERE id = @id;
