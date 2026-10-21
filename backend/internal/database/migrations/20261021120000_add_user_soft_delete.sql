-- +goose Up
-- +goose StatementBegin
-- Soft delete of users (decision D-A11). deleted_at is the only "is deleted" signal;
-- status is not touched, so a restore brings back the status the user had. Restore
-- clears all three columns; the history lives in the DELETE_USER and RESTORE_USER
-- audit rows. The CHECK change must ship in the same deploy as the code that writes
-- those actions.
ALTER TABLE users
    ADD COLUMN deleted_at     TIMESTAMPTZ NULL,
    ADD COLUMN deleted_by     BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN deleted_reason TEXT NULL CONSTRAINT chk_users_deleted_reason_length CHECK (char_length(deleted_reason) <= 500);

ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER', 'CHANGE_PASSWORD', 'CHANGE_USER_STATUS',
    'ADMIN_RESET_PASSWORD', 'DELETE_USER', 'RESTORE_USER'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- NOT VALID: rows already written with DELETE_USER or RESTORE_USER stay (the log is
-- append-only), but the old code never writes them again.
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER', 'CHANGE_PASSWORD', 'CHANGE_USER_STATUS',
    'ADMIN_RESET_PASSWORD')) NOT VALID;

ALTER TABLE users
    DROP COLUMN IF EXISTS deleted_reason,
    DROP COLUMN IF EXISTS deleted_by,
    DROP COLUMN IF EXISTS deleted_at;
-- +goose StatementEnd
