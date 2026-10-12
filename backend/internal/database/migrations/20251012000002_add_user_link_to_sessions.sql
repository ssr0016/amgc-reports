-- +goose Up
-- +goose StatementBegin
-- user_id links a session to its owner so that all of a user's sessions can be
-- revoked together (password reset, and later admin disable). The session
-- store (internal/session) fills it from the session data on every commit.
-- Deleting a user removes their sessions.
--
-- revoked_at is a tombstone, not a delete: every request re-commits its
-- session, so a request that loaded the session before the revocation would
-- recreate a deleted row. The store never finds, nor updates, a revoked row.
--
-- Sessions that already exist keep user_id NULL and cannot be revoked by user.
-- They age out with the session lifetime (24h by default, SESSION_LIFETIME).
ALTER TABLE sessions ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE sessions ADD COLUMN revoked_at TIMESTAMPTZ;
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS sessions_user_id_idx;
ALTER TABLE sessions DROP COLUMN IF EXISTS revoked_at;
ALTER TABLE sessions DROP COLUMN IF EXISTS user_id;
-- +goose StatementEnd
