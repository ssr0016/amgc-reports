-- +goose Up
-- +goose StatementBegin
ALTER TABLE reports
  ADD COLUMN last_revised_at TIMESTAMPTZ,
  ADD COLUMN user_notes TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE reports
  DROP COLUMN IF EXISTS user_notes,
  DROP COLUMN IF EXISTS last_revised_at;
-- +goose StatementEnd
