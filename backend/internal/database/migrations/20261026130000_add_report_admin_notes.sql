-- +goose Up
-- +goose StatementBegin
ALTER TABLE reports ADD COLUMN admin_notes TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE reports DROP COLUMN IF EXISTS admin_notes;
-- +goose StatementEnd
