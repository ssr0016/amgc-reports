-- +goose Up
-- +goose StatementBegin
CREATE TABLE user_notifications (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       VARCHAR(64) NOT NULL,
    message    TEXT        NOT NULL,
    metadata   JSONB       NOT NULL DEFAULT '{}'::jsonb,
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_user_notifications_type CHECK (type IN ('report_submitted'))
);

CREATE INDEX idx_user_notifications_user_read_at
    ON user_notifications (user_id, read_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_user_notifications_user_read_at;
DROP TABLE IF EXISTS user_notifications;
-- +goose StatementEnd
