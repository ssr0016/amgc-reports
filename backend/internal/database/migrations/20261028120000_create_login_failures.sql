-- +goose Up
-- Failed sign-ins are counted per account and IP (M2): a stranger who knows an
-- email can lock that email out of their own IP only, never the owner's.
-- users.failed_login_attempts and users.locked_until are no longer read; they
-- are dropped in a later migration, once no deployed code selects them.
CREATE TABLE login_failures (
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip           TEXT NOT NULL,
    attempts     INT NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, ip)
);

-- +goose Down
DROP TABLE IF EXISTS login_failures;
