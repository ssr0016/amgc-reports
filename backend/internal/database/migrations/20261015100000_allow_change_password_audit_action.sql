-- +goose Up
-- +goose StatementBegin
-- A user changing their own password is audited. The service writes this row in the
-- same transaction as the password update, so until the CHECK allows the action the
-- whole change rolls back and the endpoint answers 500.
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER', 'CHANGE_PASSWORD'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- NOT VALID enforces the old list for new rows without checking existing ones,
-- so a rollback never deletes audit history that already used CHANGE_PASSWORD.
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER')) NOT VALID;
-- +goose StatementEnd
