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
INSERT INTO announcements (title, body, type, division_id, starts_at, ends_at, is_active, created_by, min_read_seconds)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id;

-- name: UpdateAnnouncement :execrows
UPDATE announcements
SET title = $2, body = $3, type = $4, division_id = $5, starts_at = $6, ends_at = $7, is_active = $8, min_read_seconds = $9
WHERE id = $1;

-- name: DeleteAnnouncement :execrows
DELETE FROM announcements WHERE id = $1;

-- Sasaran pengumuman: Pejuang aktif di divisi tujuan, atau semua bila division_id kosong.

-- name: AnnouncementReadCounts :many
SELECT a.id,
  (SELECT count(*) FROM employees e JOIN subdivisions s ON s.id = e.subdivision_id
   WHERE e.status = 'active' AND e.deleted_at IS NULL
     AND (a.division_id IS NULL OR s.division_id = a.division_id))::bigint AS audience_count,
  (SELECT count(*) FROM announcement_reads r JOIN employees e ON e.id = r.employee_id
   JOIN subdivisions s ON s.id = e.subdivision_id
   WHERE r.announcement_id = a.id AND e.status = 'active' AND e.deleted_at IS NULL
     AND (a.division_id IS NULL OR s.division_id = a.division_id))::bigint AS opened_count,
  (SELECT count(*) FROM announcement_reads r JOIN employees e ON e.id = r.employee_id
   JOIN subdivisions s ON s.id = e.subdivision_id
   WHERE r.announcement_id = a.id AND r.read_at IS NOT NULL AND e.status = 'active' AND e.deleted_at IS NULL
     AND (a.division_id IS NULL OR s.division_id = a.division_id))::bigint AS read_count
FROM announcements a
WHERE a.id = ANY(@ids::uuid[]);

-- name: ListAnnouncementReaders :many
SELECT e.id AS employee_id, e.full_name, e.nip, s.name AS subdivision_name, r.opened_at, r.read_at
FROM employees e
JOIN subdivisions s ON s.id = e.subdivision_id
LEFT JOIN announcement_reads r ON r.employee_id = e.id AND r.announcement_id = @announcement_id
WHERE e.status = 'active' AND e.deleted_at IS NULL
  AND (sqlc.narg('division_id')::uuid IS NULL OR s.division_id = sqlc.narg('division_id')::uuid)
ORDER BY (r.read_at IS NOT NULL), (r.opened_at IS NOT NULL), e.full_name, e.id;

-- Pengumuman yang sedang tayang untuk satu Pejuang, beserta catatan buka/bacanya.

-- name: ListActiveAnnouncementsFor :many
SELECT a.id, a.title, a.body, a.type, a.starts_at, a.ends_at, a.min_read_seconds, r.opened_at, r.read_at
FROM announcements a
LEFT JOIN announcement_reads r ON r.announcement_id = a.id AND r.employee_id = @employee_id
WHERE a.is_active
  AND a.starts_at <= @now
  AND (a.ends_at IS NULL OR a.ends_at > @now)
  AND (a.division_id IS NULL OR a.division_id = (
    SELECT s.division_id FROM employees e JOIN subdivisions s ON s.id = e.subdivision_id WHERE e.id = @employee_id))
ORDER BY (r.read_at IS NOT NULL), a.starts_at DESC, a.id
LIMIT 50;

-- name: GetActiveAnnouncementFor :one
SELECT a.id, a.title, a.body, a.type, a.starts_at, a.ends_at, a.min_read_seconds, r.opened_at, r.read_at
FROM announcements a
LEFT JOIN announcement_reads r ON r.announcement_id = a.id AND r.employee_id = @employee_id
WHERE a.id = @id
  AND a.is_active
  AND a.starts_at <= @now
  AND (a.ends_at IS NULL OR a.ends_at > @now)
  AND (a.division_id IS NULL OR a.division_id = (
    SELECT s.division_id FROM employees e JOIN subdivisions s ON s.id = e.subdivision_id WHERE e.id = @employee_id));

-- name: OpenAnnouncement :exec
INSERT INTO announcement_reads (announcement_id, employee_id, opened_at)
VALUES (@announcement_id, @employee_id, @now)
ON CONFLICT (announcement_id, employee_id) DO NOTHING;

-- name: MarkAnnouncementRead :exec
UPDATE announcement_reads
SET read_at = @now
WHERE announcement_id = @announcement_id AND employee_id = @employee_id AND read_at IS NULL;
