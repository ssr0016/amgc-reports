-- +goose Up
-- +goose StatementBegin
-- Sign-ins, sign-outs and refused sign-ins are audited (D-A4, amended 2026-10-22).
-- A successful login writes LOGIN in the same transaction as its refresh token, so
-- until the CHECK allows the action every login rolls back and answers 500. The
-- CHECK change must ship in the same deploy as the code that writes it.
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER', 'CHANGE_PASSWORD', 'CHANGE_USER_STATUS',
    'ADMIN_RESET_PASSWORD', 'DELETE_USER', 'RESTORE_USER',
    'LOGIN', 'LOGOUT', 'LOGIN_FAILED'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- NOT VALID: rows already written with LOGIN, LOGOUT or LOGIN_FAILED stay (the log
-- is append-only), but the old code never writes them again.
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER', 'CHANGE_PASSWORD', 'CHANGE_USER_STATUS',
    'ADMIN_RESET_PASSWORD', 'DELETE_USER', 'RESTORE_USER')) NOT VALID;
-- +goose StatementEnd
