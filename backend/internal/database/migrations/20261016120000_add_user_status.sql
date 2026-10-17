-- +goose Up
-- +goose StatementBegin
-- Account status (decision D-A10). Only an active account can sign in or keep a
-- session; the other three are set by an admin. Existing rows become active
-- through the default, so the previous code keeps working until it is replaced.
ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE users ADD CONSTRAINT chk_users_status
    CHECK (status IN ('active', 'inactive', 'disciplinary', 'deceased'));

-- A status change is audited in the same transaction (chk_audit_logs_action
-- CHECK, so a new action needs a migration shipped with the code that writes it).
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER', 'CHANGE_PASSWORD', 'CHANGE_USER_STATUS'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- NOT VALID enforces the old list on new rows without checking existing ones,
-- so a rollback never deletes audit history that already holds CHANGE_USER_STATUS.
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER', 'CHANGE_PASSWORD')) NOT VALID;

ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_status;
ALTER TABLE users DROP COLUMN IF EXISTS status;
-- +goose StatementEnd
