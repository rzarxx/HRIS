-- Role kepala yang ditetapkan otomatis (D49) tidak dikembalikan; ubah manual bila perlu.
DROP TABLE IF EXISTS leave_requests;
DROP TABLE IF EXISTS leave_types;
DROP TABLE IF EXISTS approvals;
DROP TABLE IF EXISTS approval_flow_steps;
DROP TYPE IF EXISTS leave_quota_kind;
DROP TYPE IF EXISTS approval_step_status;
DROP TYPE IF EXISTS request_status;
DROP TYPE IF EXISTS request_kind;
