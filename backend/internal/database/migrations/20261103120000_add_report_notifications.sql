-- +goose Up
-- +goose StatementBegin
-- Report notification preferences are nullable by design. NULL and missing
-- event keys both mean notifications are enabled.
ALTER TABLE users
    ADD COLUMN notification_prefs JSONB;

-- Durable report notification work. Report and user references are nullable so
-- hard deletion preserves the outbox row and its recipient snapshot.
CREATE TABLE report_notification_outbox (
    id                  BIGSERIAL PRIMARY KEY,
    idempotency_key     VARCHAR(255) NOT NULL UNIQUE,
    event_type          VARCHAR(32)  NOT NULL,
    report_id           BIGINT REFERENCES reports(id) ON DELETE SET NULL,
    recipient_user_id   BIGINT REFERENCES users(id) ON DELETE SET NULL,
    recipient_email     VARCHAR(320) NOT NULL,
    report_month        VARCHAR(9)   NOT NULL,
    report_year         SMALLINT     NOT NULL,
    admin_remark        TEXT,
    state               VARCHAR(16)  NOT NULL DEFAULT 'pending',
    attempts            INTEGER      NOT NULL DEFAULT 0,
    next_attempt_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    claimed_at          TIMESTAMPTZ,
    sent_at             TIMESTAMPTZ,
    skipped_at          TIMESTAMPTZ,
    skip_reason         TEXT,
    last_error          TEXT,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_report_notification_event_type
        CHECK (event_type IN ('approved', 'needs_revision')),
    CONSTRAINT chk_report_notification_state
        CHECK (state IN ('pending', 'claimed', 'sent', 'skipped', 'failed')),
    CONSTRAINT chk_report_notification_attempts
        CHECK (attempts >= 0)
);

CREATE INDEX idx_report_notification_outbox_due
    ON report_notification_outbox (state, next_attempt_at);
CREATE INDEX idx_report_notification_outbox_claimed
    ON report_notification_outbox (claimed_at)
    WHERE state = 'claimed';
CREATE INDEX idx_report_notification_outbox_report
    ON report_notification_outbox (report_id);
CREATE INDEX idx_report_notification_outbox_recipient
    ON report_notification_outbox (recipient_user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_report_notification_outbox_recipient;
DROP INDEX IF EXISTS idx_report_notification_outbox_report;
DROP INDEX IF EXISTS idx_report_notification_outbox_claimed;
DROP INDEX IF EXISTS idx_report_notification_outbox_due;
DROP TABLE IF EXISTS report_notification_outbox;
ALTER TABLE users DROP COLUMN IF EXISTS notification_prefs;
-- +goose StatementEnd
