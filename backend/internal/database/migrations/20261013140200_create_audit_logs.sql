-- +goose Up
-- +goose StatementBegin
-- Audit trail of report actions (decision D-10). Each row is written in the
-- same transaction as the change it records. Actor name and role are snapshots,
-- so the log stays meaningful if the user or role changes. details holds
-- {month, year, count}; narrative and prayer text must never be stored there.
-- The action list is a CHECK, so a new action needs a migration.
CREATE TABLE audit_logs (
    id                BIGSERIAL    PRIMARY KEY,
    performed_by      BIGINT       REFERENCES users(id) ON DELETE SET NULL,
    performed_by_name VARCHAR(100),
    performed_by_role VARCHAR(50),
    action            VARCHAR(50)  NOT NULL,
    target_type       VARCHAR(50)  NOT NULL,
    target_id         VARCHAR(64),
    target_name       VARCHAR(255),
    details           JSONB        NOT NULL DEFAULT '{}'::jsonb,
    ip_address        VARCHAR(64),
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_audit_logs_action CHECK (action IN (
        'CREATE_REPORT', 'UPDATE_REPORT', 'DELETE_REPORT',
        'APPROVE_REPORT', 'UNAPPROVE_REPORT',
        'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL'))
);

CREATE INDEX idx_audit_logs_created_at ON audit_logs (created_at DESC);
CREATE INDEX idx_audit_logs_target     ON audit_logs (target_type, target_id);
CREATE INDEX idx_audit_logs_action     ON audit_logs (action);

-- Append-only for the server: the default privileges grant DML on new tables,
-- so take back the two that could rewrite history. Deleting an audited user
-- still works: the SET NULL action runs with the table owner's rights.
REVOKE UPDATE, DELETE ON audit_logs FROM app_user;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_logs;
-- +goose StatementEnd
