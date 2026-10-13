-- +goose Up
-- +goose StatementBegin
-- Weekly metric values of a report (decisions D-5, D-6). Sparse: a row exists
-- only for a (metric, week) that was reported, so a missing row means "not
-- reported" and 0 means "reported zero". Rows are replaced as a set on update.
-- Adding a metric needs a migration that replaces chk_report_metrics_metric.
-- Soft-deleting a report leaves its metrics; CASCADE only applies to a hard delete.
CREATE TABLE report_metrics (
    report_id BIGINT        NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    metric    VARCHAR(50)   NOT NULL,
    week_no   SMALLINT      NOT NULL,
    value     NUMERIC(12,2) NOT NULL,
    PRIMARY KEY (report_id, metric, week_no),

    CONSTRAINT chk_report_metrics_metric CHECK (metric IN (
        'worship_service', 'sunday_school', 'prayer_meeting', 'bible_studies',
        'mens_fellowship', 'womens_fellowship', 'youth_fellowship', 'children_fellowship',
        'tithes_offering', 'home_visited', 'bible_study_group_led', 'sermon_preached',
        'person_newly_contacted', 'person_followed_up', 'person_evangelized', 'outreach',
        'training', 'leadership', 'baptism', 'other', 'family_day')),
    CONSTRAINT chk_report_metrics_week  CHECK (week_no BETWEEN 1 AND 5),
    CONSTRAINT chk_report_metrics_value CHECK (value >= 0)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS report_metrics;
-- +goose StatementEnd
