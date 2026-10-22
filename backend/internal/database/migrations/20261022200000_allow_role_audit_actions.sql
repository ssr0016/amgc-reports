-- +goose Up
-- +goose StatementBegin
-- Role changes are audited (D-A21): creating, updating and deleting a role, and
-- changing a user's role. Each row is written in the same transaction as the change,
-- so until the CHECK allows the action every such change rolls back and answers 500.
-- The CHECK change must ship in the same deploy as the code that writes it.
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER', 'CHANGE_PASSWORD', 'CHANGE_USER_STATUS',
    'ADMIN_RESET_PASSWORD', 'DELETE_USER', 'RESTORE_USER',
    'LOGIN', 'LOGOUT', 'LOGIN_FAILED',
    'CREATE_ROLE', 'UPDATE_ROLE', 'DELETE_ROLE', 'CHANGE_USER_ROLE'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- NOT VALID: rows already written with the role actions stay (the log is
-- append-only), but the old code never writes them again.
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER', 'CHANGE_PASSWORD', 'CHANGE_USER_STATUS',
    'ADMIN_RESET_PASSWORD', 'DELETE_USER', 'RESTORE_USER',
    'LOGIN', 'LOGOUT', 'LOGIN_FAILED')) NOT VALID;
-- +goose StatementEnd
