-- name: ListPayrollComponents :many
SELECT * FROM payroll_components ORDER BY sort_order, name;

-- name: ListActivePayrollComponents :many
SELECT * FROM payroll_components WHERE active ORDER BY sort_order, name;

-- name: GetPayrollComponent :one
SELECT * FROM payroll_components WHERE id = $1;

-- name: InsertPayrollComponent :one
INSERT INTO payroll_components (code, name, kind, method, rate, percent, divisor, active, show_on_slip, sort_order, note, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: UpdatePayrollComponent :one
UPDATE payroll_components
SET name = $2, kind = $3, method = $4, rate = $5, percent = $6, divisor = $7, active = $8, show_on_slip = $9,
    sort_order = $10, note = $11, updated_by = $12, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListEmployeePayrollComponents :many
SELECT epc.component_id, epc.amount
FROM employee_payroll_components epc
WHERE epc.employee_id = $1;

-- name: ListAllEmployeePayrollComponents :many
SELECT epc.employee_id, epc.component_id, epc.amount
FROM employee_payroll_components epc
JOIN payroll_components c ON c.id = epc.component_id
WHERE c.active AND c.method = 'per_employee';

-- name: DeleteEmployeePayrollComponents :exec
DELETE FROM employee_payroll_components WHERE employee_id = $1;

-- name: InsertEmployeePayrollComponent :exec
INSERT INTO employee_payroll_components (employee_id, component_id, amount, updated_by)
VALUES ($1, $2, $3, $4);


-- Periode ubar (D67) ---------------------------------------------------------------------------------

-- name: ListPayrollPeriods :many
SELECT p.*,
       (SELECT count(*) FROM payrolls r WHERE r.period_id = p.id) AS employee_count,
       (SELECT COALESCE(sum(r.net), 0) FROM payrolls r WHERE r.period_id = p.id)::numeric(16,2) AS total_net
FROM payroll_periods p
ORDER BY p.year DESC, p.month DESC;

-- name: GetPayrollPeriod :one
SELECT * FROM payroll_periods WHERE id = $1;

-- name: LockPayrollPeriod :one
SELECT * FROM payroll_periods WHERE id = $1 FOR UPDATE;

-- name: LatestPayrollPeriod :one
SELECT * FROM payroll_periods ORDER BY end_date DESC LIMIT 1;

-- name: PayrollPeriodOverlaps :one
SELECT EXISTS (SELECT 1 FROM payroll_periods WHERE start_date <= @end_date AND end_date >= @start_date);

-- name: InsertPayrollPeriod :one
INSERT INTO payroll_periods (year, month, start_date, end_date, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: DeletePayrollPeriod :exec
DELETE FROM payroll_periods WHERE id = $1 AND status = 'draft';

-- name: SetPayrollPeriodGenerated :exec
UPDATE payroll_periods
SET rules_snapshot = $2, generated_by = $3, generated_at = $4, reject_note = NULL, updated_at = now()
WHERE id = $1;

-- name: SetPayrollPeriodSubmitted :exec
UPDATE payroll_periods SET status = 'pending_approval', submitted_by = $2, submitted_at = $3, updated_at = now() WHERE id = $1;

-- name: SetPayrollPeriodRejected :exec
UPDATE payroll_periods
SET status = 'draft', reject_note = $2, rejected_by = $3, rejected_at = $4, submitted_by = NULL, submitted_at = NULL, updated_at = now()
WHERE id = $1;

-- name: SetPayrollPeriodFinalized :exec
UPDATE payroll_periods SET status = 'final', finalized_by = $2, finalized_at = $3, updated_at = now() WHERE id = $1;

-- name: SetPayrollPeriodPaid :exec
UPDATE payroll_periods SET status = 'paid', paid_by = $2, paid_at = $3, updated_at = now() WHERE id = $1;

-- name: PublishPayrolls :exec
UPDATE payrolls SET published_at = $2 WHERE period_id = $1;

-- name: DeletePayrollsOfPeriod :exec
DELETE FROM payrolls WHERE period_id = $1;

-- name: DeletePayroll :exec
DELETE FROM payrolls WHERE period_id = $1 AND employee_id = $2;

-- name: InsertPayroll :one
INSERT INTO payrolls (period_id, employee_id, nip, full_name, division, subdivision, position, base_salary, gross, deductions,
                      net, net_clamped, attendance, employee_values)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING id;

-- name: InsertPayrollItem :exec
INSERT INTO payroll_items (payroll_id, position, component_code, name, kind, quantity, rate, amount, show_on_slip, manual)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListPayrollsOfPeriod :many
SELECT * FROM payrolls WHERE period_id = $1 ORDER BY full_name, employee_id;

-- name: GetPayrollOfEmployee :one
SELECT * FROM payrolls WHERE period_id = $1 AND employee_id = $2;

-- name: ListPayrollItems :many
SELECT * FROM payroll_items WHERE payroll_id = $1 ORDER BY position;

-- name: ListPayrollItemsOfPeriod :many
SELECT i.* FROM payroll_items i JOIN payrolls r ON r.id = i.payroll_id WHERE r.period_id = $1 ORDER BY i.payroll_id, i.position;

-- name: ListPayrollInputsOfPeriod :many
SELECT * FROM payroll_inputs WHERE period_id = $1;

-- name: ListPayrollInputsOfEmployee :many
SELECT * FROM payroll_inputs WHERE period_id = $1 AND employee_id = $2;

-- name: DeletePayrollInputsOfEmployee :exec
DELETE FROM payroll_inputs WHERE period_id = $1 AND employee_id = $2;

-- name: InsertPayrollInput :exec
INSERT INTO payroll_inputs (period_id, employee_id, component_id, quantity, amount, note, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: PayrollEmployeeInfo :many
-- Identitas dan ubar pokok Pejuang untuk salinan slip.
SELECT e.id, e.nip, e.full_name, COALESCE(e.position, '') AS position, e.base_salary, d.name AS division, s.name AS subdivision
FROM employees e
JOIN subdivisions s ON s.id = e.subdivision_id
JOIN divisions d ON d.id = s.division_id
WHERE e.id = ANY(@ids::uuid[]);

-- name: AttendanceDateLocked :one
-- Presensi di periode yang sudah diajukan, final, atau dibayar tidak boleh diubah (PERIOD_LOCKED).
SELECT EXISTS (
  SELECT 1 FROM payroll_periods
  WHERE status <> 'draft' AND start_date <= @last_date::date AND end_date >= @first_date::date
);
