-- +goose Up
-- +goose StatementBegin
-- Each of these repeats the index Postgres already builds for the column's UNIQUE
-- constraint (users_email_key, roles_name_key, ...): same column, same btree. Lookups
-- keep using the constraint's index; dropping these only saves the second write.
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_roles_name;
DROP INDEX IF EXISTS idx_permissions_name;
DROP INDEX IF EXISTS idx_password_resets_token;
DROP INDEX IF EXISTS idx_verification_tokens_token;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_roles_name ON roles(name);
CREATE INDEX IF NOT EXISTS idx_permissions_name ON permissions(name);
CREATE INDEX IF NOT EXISTS idx_password_resets_token ON password_resets(token);
CREATE INDEX IF NOT EXISTS idx_verification_tokens_token ON verification_tokens(token);
-- +goose StatementEnd
