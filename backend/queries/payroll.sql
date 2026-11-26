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

