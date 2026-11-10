-- name: ListFlowSteps :many
SELECT * FROM approval_flow_steps WHERE request_type = $1 ORDER BY step_order;

-- name: DeleteFlowSteps :exec
DELETE FROM approval_flow_steps WHERE request_type = $1;

-- name: InsertFlowStep :exec
INSERT INTO approval_flow_steps (request_type, step_order, approver_role) VALUES ($1, $2, $3);

-- Kepala subdiv dan divisi pemohon. Kepala yang akunnya nonaktif dianggap kosong.
-- name: GetRequesterHeads :one
SELECT e.id AS employee_id, su.employee_id AS subdivision_head_id, du.employee_id AS division_head_id
FROM employees e
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
LEFT JOIN users su ON su.employee_id = s.head_employee_id AND su.is_active
LEFT JOIN users du ON du.employee_id = d.head_employee_id AND du.is_active
WHERE e.id = $1 AND e.deleted_at IS NULL;

-- name: InsertApproval :one
INSERT INTO approvals (request_type, request_id, requester_employee_id, step_order, approver_role, assigned_employee_id, status, note)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListApprovalsForRequest :many
SELECT a.*, e.full_name AS approver_name
FROM approvals a
LEFT JOIN employees e ON e.id = a.assigned_employee_id
WHERE a.request_type = $1 AND a.request_id = $2
ORDER BY a.step_order;

-- name: ListApprovalsForRequests :many
SELECT a.*, e.full_name AS approver_name
FROM approvals a
LEFT JOIN employees e ON e.id = a.assigned_employee_id
WHERE a.request_type = $1 AND a.request_id = ANY(@request_ids::uuid[])
ORDER BY a.request_id, a.step_order;

-- name: LockApproval :one
SELECT * FROM approvals WHERE id = $1 FOR UPDATE;

-- name: LockApprovalsForRequest :many
SELECT * FROM approvals WHERE request_type = $1 AND request_id = $2 ORDER BY step_order FOR UPDATE;

-- name: SetApprovalDecision :exec
UPDATE approvals SET status = @status, note = @note, acted_by = @acted_by, acted_at = @acted_at WHERE id = @id;

-- name: ActivateApproval :exec
UPDATE approvals SET status = 'pending' WHERE id = $1 AND status = 'waiting';

-- name: SkipOpenApprovals :exec
UPDATE approvals SET status = 'skipped'
WHERE request_type = $1 AND request_id = $2 AND status IN ('waiting', 'pending');

-- Langkah yang menunggu keputusan orang ini: ditugaskan kepadanya, atau pool sdm bila ia SDM.
-- Pengajuan sendiri tidak pernah tampil (APPROVAL_SELF_FORBIDDEN).
-- name: ListPendingApprovals :many
SELECT a.*, e.full_name AS requester_name, s.name AS requester_subdivision
FROM approvals a
JOIN employees e ON e.id = a.requester_employee_id
JOIN subdivisions s ON s.id = e.subdivision_id
WHERE a.status = 'pending'
  AND a.requester_employee_id <> @employee_id
  AND (a.assigned_employee_id = @employee_id OR (a.assigned_employee_id IS NULL AND @is_sdm::boolean))
ORDER BY a.created_at, a.step_order;

-- name: GetApprovalDetail :one
SELECT a.*, e.full_name AS requester_name, s.name AS requester_subdivision
FROM approvals a
JOIN employees e ON e.id = a.requester_employee_id
JOIN subdivisions s ON s.id = e.subdivision_id
WHERE a.id = $1;
