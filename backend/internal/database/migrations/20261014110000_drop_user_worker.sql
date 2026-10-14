-- +goose Up
-- +goose StatementBegin
-- A report's worker defaults to the user's name, so a separate worker on the
-- account only repeated it. The report keeps its own worker column. This drops
-- whatever was stored in users.worker.
ALTER TABLE users DROP COLUMN worker;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- The values are not recoverable; the column comes back empty.
ALTER TABLE users ADD COLUMN worker VARCHAR(150);
-- +goose StatementEnd
