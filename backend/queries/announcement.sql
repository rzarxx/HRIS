-- name: ListAnnouncements :many
SELECT a.*, d.name AS division_name
FROM announcements a
LEFT JOIN divisions d ON d.id = a.division_id
ORDER BY a.starts_at DESC, a.id
LIMIT @page_limit OFFSET @page_offset;

-- name: CountAnnouncements :one
SELECT count(*) FROM announcements;

-- name: GetAnnouncement :one
SELECT a.*, d.name AS division_name
FROM announcements a
LEFT JOIN divisions d ON d.id = a.division_id
WHERE a.id = $1;

-- name: CreateAnnouncement :one
INSERT INTO announcements (title, body, type, division_id, starts_at, ends_at, is_active, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id;

-- name: UpdateAnnouncement :execrows
UPDATE announcements
SET title = $2, body = $3, type = $4, division_id = $5, starts_at = $6, ends_at = $7, is_active = $8
WHERE id = $1;

-- name: DeleteAnnouncement :execrows
DELETE FROM announcements WHERE id = $1;
