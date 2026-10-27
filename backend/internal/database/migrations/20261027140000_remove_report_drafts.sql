-- +goose Up
-- +goose StatementBegin
-- Drafts are removed (see DECISIONS.md); an unsubmitted draft becomes an open report.
UPDATE reports SET status = 'pending' WHERE status = 'draft';
ALTER TABLE reports DROP CONSTRAINT chk_reports_status;
ALTER TABLE reports ADD CONSTRAINT chk_reports_status
    CHECK (status IN ('pending', 'approved', 'needs_revision'));
-- SUBMIT_REPORT stays in chk_audit_logs_action: historical rows keep it.
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Former drafts cannot be told apart from pending reports; only the constraint is restored.
ALTER TABLE reports DROP CONSTRAINT chk_reports_status;
ALTER TABLE reports ADD CONSTRAINT chk_reports_status
    CHECK (status IN ('draft', 'pending', 'approved', 'needs_revision'));
-- +goose StatementEnd
