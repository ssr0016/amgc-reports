-- +goose Up
-- +goose StatementBegin
-- A church is the normalized church_name its accounts and reports carry
-- (lower(btrim(name)), D-A24). This table holds what belongs to the church, not to
-- a worker account: its reporting eligibility (D-A31). Months are activity months,
-- stored as their first day. A NULL start means the admin has not reviewed it yet,
-- so no month is expected (none is shown as missing); a NULL end means open-ended.
CREATE TABLE churches (
    id                    BIGSERIAL    PRIMARY KEY,
    name                  VARCHAR(150) NOT NULL,
    name_key              VARCHAR(150) NOT NULL UNIQUE,
    reporting_start_month DATE,
    reporting_end_month   DATE,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_churches_name_key CHECK (name_key = lower(btrim(name)) AND name_key <> ''),
    CONSTRAINT chk_churches_start_first CHECK (EXTRACT(DAY FROM reporting_start_month) = 1),
    CONSTRAINT chk_churches_end_first CHECK (EXTRACT(DAY FROM reporting_end_month) = 1),
    CONSTRAINT chk_churches_period CHECK (reporting_end_month >= reporting_start_month)
);

-- Every church name an account or report carries today; dates are left for review.
INSERT INTO churches (name, name_key)
SELECT MIN(btrim(n)), lower(btrim(n)) FROM (
    SELECT church_name AS n FROM users
    UNION ALL
    SELECT church_name FROM reports WHERE deleted_at IS NULL
) names
WHERE btrim(COALESCE(n, '')) <> ''
GROUP BY lower(btrim(n));

-- A new church name on an account gets its church row, so analytics always has one.
CREATE FUNCTION ensure_church() RETURNS trigger LANGUAGE plpgsql AS $fn$
BEGIN
    IF btrim(COALESCE(NEW.church_name, '')) <> '' THEN
        INSERT INTO churches (name, name_key)
        VALUES (btrim(NEW.church_name), lower(btrim(NEW.church_name)))
        ON CONFLICT (name_key) DO NOTHING;
    END IF;
    RETURN NEW;
END
$fn$;
CREATE TRIGGER trg_users_ensure_church AFTER INSERT OR UPDATE OF church_name ON users
    FOR EACH ROW EXECUTE FUNCTION ensure_church();

-- Setting a church's reporting months is audited (UPDATE_CHURCH_REPORTING).
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'SUBMIT_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER', 'CHANGE_PASSWORD', 'CHANGE_USER_STATUS',
    'ADMIN_RESET_PASSWORD', 'DELETE_USER', 'RESTORE_USER',
    'LOGIN', 'LOGOUT', 'LOGIN_FAILED',
    'CREATE_ROLE', 'UPDATE_ROLE', 'DELETE_ROLE', 'CHANGE_USER_ROLE',
    'UPDATE_CHURCH_REPORTING'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE audit_logs DROP CONSTRAINT chk_audit_logs_action;
ALTER TABLE audit_logs ADD CONSTRAINT chk_audit_logs_action CHECK (action IN (
    'CREATE_REPORT', 'UPDATE_REPORT', 'SUBMIT_REPORT', 'DELETE_REPORT',
    'APPROVE_REPORT', 'UNAPPROVE_REPORT',
    'DOWNLOAD_SINGLE_EXCEL', 'DOWNLOAD_BULK_EXCEL',
    'CREATE_USER', 'UPDATE_USER', 'CHANGE_PASSWORD', 'CHANGE_USER_STATUS',
    'ADMIN_RESET_PASSWORD', 'DELETE_USER', 'RESTORE_USER',
    'LOGIN', 'LOGOUT', 'LOGIN_FAILED',
    'CREATE_ROLE', 'UPDATE_ROLE', 'DELETE_ROLE', 'CHANGE_USER_ROLE')) NOT VALID;
DROP TRIGGER IF EXISTS trg_users_ensure_church ON users;
DROP FUNCTION IF EXISTS ensure_church();
DROP TABLE IF EXISTS churches;
-- +goose StatementEnd
