-- +goose Up
-- +goose StatementBegin
ALTER TABLE reports DROP CONSTRAINT chk_reports_status;
ALTER TABLE reports ADD CONSTRAINT chk_reports_status
    CHECK (status IN ('draft', 'pending', 'approved', 'needs_revision'));

-- Submission is audited in the same transaction as the status transition.
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'SUBMIT_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER', 'CHANGE_PASSWORD', 'CHANGE_USER_STATUS',
    'ADMIN_RESET_PASSWORD', 'DELETE_USER', 'RESTORE_USER',
    'LOGIN', 'LOGOUT', 'LOGIN_FAILED',
    'CREATE_ROLE', 'UPDATE_ROLE', 'DELETE_ROLE', 'CHANGE_USER_ROLE')) NOT VALID;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER', 'CHANGE_PASSWORD', 'CHANGE_USER_STATUS',
    'ADMIN_RESET_PASSWORD', 'DELETE_USER', 'RESTORE_USER',
    'LOGIN', 'LOGOUT', 'LOGIN_FAILED',
    'CREATE_ROLE', 'UPDATE_ROLE', 'DELETE_ROLE', 'CHANGE_USER_ROLE')) NOT VALID;
ALTER TABLE reports DROP CONSTRAINT chk_reports_status;
ALTER TABLE reports ADD CONSTRAINT chk_reports_status
    CHECK (status IN ('pending', 'approved', 'needs_revision'));
-- +goose StatementEnd
