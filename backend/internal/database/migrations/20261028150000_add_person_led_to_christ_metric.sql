-- +goose Up
-- +goose StatementBegin
-- "Persons Led to Christ" gets its own key (owner, D-A31). person_evangelized stays
-- for historical rows, labelled "Legacy Persons Evangelized — meaning unverified":
-- its entries are never re-read as persons led to Christ.
ALTER TABLE report_metrics DROP CONSTRAINT chk_report_metrics_metric;
ALTER TABLE report_metrics ADD CONSTRAINT chk_report_metrics_metric CHECK (metric IN (
    'worship_service', 'sunday_school', 'prayer_meeting', 'bible_studies',
    'mens_fellowship', 'womens_fellowship', 'youth_fellowship', 'children_fellowship',
    'tithes_offering', 'home_visited', 'bible_study_group_led', 'sermon_preached',
    'person_newly_contacted', 'person_followed_up', 'person_led_to_christ', 'outreach',
    'training', 'leadership', 'baptism', 'other', 'family_day', 'person_evangelized'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- NOT VALID: rows already holding person_led_to_christ stay; the old code never writes it.
ALTER TABLE report_metrics DROP CONSTRAINT chk_report_metrics_metric;
ALTER TABLE report_metrics ADD CONSTRAINT chk_report_metrics_metric CHECK (metric IN (
    'worship_service', 'sunday_school', 'prayer_meeting', 'bible_studies',
    'mens_fellowship', 'womens_fellowship', 'youth_fellowship', 'children_fellowship',
    'tithes_offering', 'home_visited', 'bible_study_group_led', 'sermon_preached',
    'person_newly_contacted', 'person_followed_up', 'person_evangelized', 'outreach',
    'training', 'leadership', 'baptism', 'other', 'family_day')) NOT VALID;
-- +goose StatementEnd
