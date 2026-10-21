-- +goose Up
-- +goose StatementBegin
-- JWT sign-in (decision D-AR1). The access token is stateless; what can be
-- revoked is the refresh token. Each sign-in starts a family (family_id, carried
-- in the access token as sid); every refresh replaces the token by a new one in
-- the same family. Only the SHA-256 of the token is stored, so a database leak
-- does not yield a usable token.
--
-- expires_at is absolute for the family (a new row inherits it). last_used_at
-- drives the idle timeout. replaced_by marks a rotated token: presenting one
-- again later means it was copied, and the whole family is revoked.
CREATE TABLE refresh_tokens (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    family_id    TEXT        NOT NULL,
    token_hash   TEXT        NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at   TIMESTAMPTZ,
    replaced_by  BIGINT REFERENCES refresh_tokens(id) ON DELETE SET NULL,
    ip           TEXT,
    user_agent   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX refresh_tokens_user_live_idx ON refresh_tokens (user_id) WHERE revoked_at IS NULL;
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);

-- An access token issued before this moment is refused (password change, admin
-- reset, status change). The epoch default means "never revoked".
ALTER TABLE users ADD COLUMN tokens_valid_after TIMESTAMPTZ NOT NULL DEFAULT 'epoch';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS tokens_valid_after;
DROP TABLE IF EXISTS refresh_tokens;
-- +goose StatementEnd
