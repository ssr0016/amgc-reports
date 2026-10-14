-- +goose Up
-- +goose StatementBegin
-- Defaults for the report form: the admin sets where a pastor works, and a new
-- report starts from these values. They are nullable (an account may have none)
-- and use the same 150-character limit as the report columns they fill.
-- Nothing filters on them, so no index.
ALTER TABLE users
    ADD COLUMN worker          VARCHAR(150),
    ADD COLUMN area_assignment VARCHAR(150),
    ADD COLUMN church_name     VARCHAR(150);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users
    DROP COLUMN IF EXISTS church_name,
    DROP COLUMN IF EXISTS area_assignment,
    DROP COLUMN IF EXISTS worker;
-- +goose StatementEnd
