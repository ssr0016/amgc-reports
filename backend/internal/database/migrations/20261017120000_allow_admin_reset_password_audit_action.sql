-- +goose Up
-- +goose StatementBegin
-- An admin resetting another user's password is audited (decision D-A12). The service
-- writes this row in the same transaction as the password update, so until the CHECK
-- allows the action the whole reset rolls back and the endpoint answers 500. The CHECK
-- change must ship in the same deploy as the code that writes the action.
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER', 'CHANGE_PASSWORD', 'CHANGE_USER_STATUS',
    'ADMIN_RESET_PASSWORD'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- NOT VALID: rows already written with ADMIN_RESET_PASSWORD stay (the log is
-- append-only), but the old code never writes it again.
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER', 'CHANGE_PASSWORD', 'CHANGE_USER_STATUS')) NOT VALID;
-- +goose StatementEnd
