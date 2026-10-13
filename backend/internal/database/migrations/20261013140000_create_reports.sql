-- +goose Up
-- +goose StatementBegin
-- Monthly pastoral reports (decisions D-3, D-7, D-11 in docs/07-Reports-Decisions.md).
-- Soft delete: every read must filter deleted_at IS NULL. created_by is
-- RESTRICT, so a user who authored a report (even a soft-deleted one) cannot be
-- deleted. Text lengths count characters, matching the API validation.
-- worker, area_assignment and church_name are filtered with ILIKE '%x%', which
-- a btree cannot serve, so they have no index.
CREATE TABLE reports (
    id               BIGSERIAL    PRIMARY KEY,
    created_by       BIGINT       NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    month            VARCHAR(9)   NOT NULL,
    year             SMALLINT     NOT NULL,
    worker           VARCHAR(150),
    area_assignment  VARCHAR(150),
    church_name      VARCHAR(150),
    names            TEXT,
    narrative_report TEXT,
    challenges       TEXT,
    prayer_request   TEXT,
    completed        BOOLEAN      NOT NULL DEFAULT FALSE,
    completed_at     TIMESTAMPTZ,
    completed_by     BIGINT       REFERENCES users(id) ON DELETE SET NULL,
    deleted_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_reports_month CHECK (month IN (
        'January', 'February', 'March', 'April', 'May', 'June',
        'July', 'August', 'September', 'October', 'November', 'December')),
    CONSTRAINT chk_reports_year CHECK (year BETWEEN 2000 AND 2100),
    CONSTRAINT chk_reports_completed CHECK (
        (completed AND completed_at IS NOT NULL) OR (NOT completed AND completed_at IS NULL)),
    CONSTRAINT chk_reports_text_len CHECK (
        char_length(names)            <= 5000  AND
        char_length(narrative_report) <= 10000 AND
        char_length(challenges)       <= 10000 AND
        char_length(prayer_request)   <= 10000)
);

-- One active report per user, month and year. A soft-deleted report is outside
-- the index, so the same period can be filed again.
CREATE UNIQUE INDEX idx_reports_unique_per_user
    ON reports (created_by, month, year) WHERE deleted_at IS NULL;
CREATE INDEX idx_reports_year_created_at
    ON reports (year DESC, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_reports_year_month
    ON reports (year, month) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS reports;
-- +goose StatementEnd
