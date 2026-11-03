-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ALTER COLUMN notification_prefs SET DEFAULT '{"daily_digest": true}'::jsonb;
UPDATE users SET notification_prefs = COALESCE(notification_prefs, '{}'::jsonb) || '{"daily_digest": true}'::jsonb
WHERE notification_prefs IS NULL OR NOT (notification_prefs ? 'daily_digest');
ALTER TABLE report_notification_outbox ADD COLUMN payload JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE report_notification_outbox DROP CONSTRAINT chk_report_notification_event_type;
ALTER TABLE report_notification_outbox ADD CONSTRAINT chk_report_notification_event_type
    CHECK (event_type IN ('approved', 'needs_revision', 'daily_digest'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE report_notification_outbox DROP CONSTRAINT chk_report_notification_event_type;
ALTER TABLE report_notification_outbox ADD CONSTRAINT chk_report_notification_event_type
    CHECK (event_type IN ('approved', 'needs_revision'));
ALTER TABLE report_notification_outbox DROP COLUMN payload;
ALTER TABLE users ALTER COLUMN notification_prefs DROP DEFAULT;
-- +goose StatementEnd
