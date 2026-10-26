-- +goose Up
-- +goose StatementBegin
ALTER TABLE reports ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'pending';
UPDATE reports SET status = 'approved' WHERE completed;
ALTER TABLE reports ADD CONSTRAINT chk_reports_status CHECK (status IN ('pending', 'approved', 'needs_revision'));
ALTER TABLE reports ADD CONSTRAINT chk_reports_status_completed CHECK ((status = 'approved') = completed);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE reports DROP CONSTRAINT IF EXISTS chk_reports_status_completed;
ALTER TABLE reports DROP CONSTRAINT IF EXISTS chk_reports_status;
ALTER TABLE reports DROP COLUMN IF EXISTS status;
-- +goose StatementEnd
