-- name: GetAppSetting :one
SELECT key, value, updated_by, updated_at FROM app_settings WHERE key = $1;

-- name: UpsertAppSetting :exec
INSERT INTO app_settings (key, value, updated_by, updated_at)
VALUES (@key, @value, @updated_by, now())
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now();

-- name: DeleteAppSetting :execrows
DELETE FROM app_settings WHERE key = $1;
