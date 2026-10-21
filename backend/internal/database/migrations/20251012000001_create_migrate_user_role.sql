-- +goose Up
-- +goose StatementBegin
-- Skipped on Neon: it blocks ALTER DEFAULT PRIVILEGES, so this migration (which
-- created the migrate_user role and handed it ownership of the tables) cannot
-- run there. It is a deliberate no-op. The version stays in the list so that
-- databases that already applied it keep a consistent history.
SELECT 1;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
