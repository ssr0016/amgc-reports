-- +goose Up
-- +goose StatementBegin
-- Account creation and profile edits are audited like report actions
-- (chk_audit_logs_action is a CHECK, so a new action needs a migration).
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- NOT VALID enforces the old list for new rows without checking existing ones,
-- so a rollback never deletes audit history that already used the user actions.
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL')) NOT VALID;
-- +goose StatementEnd
