-- +goose Up
-- +goose StatementBegin
-- One organization-wide reporting start (owner, D-A32): the first activity month every
-- church is expected to report, unless the church overrides it. Confirmed by the owner
-- as September 2026; it is not inferred from accounts or reports. One row only.
CREATE TABLE reporting_settings (
    id                  BOOLEAN     PRIMARY KEY DEFAULT TRUE CONSTRAINT chk_reporting_settings_single CHECK (id),
    default_start_month DATE        NOT NULL CONSTRAINT chk_reporting_settings_first CHECK (EXTRACT(DAY FROM default_start_month) = 1),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO reporting_settings (default_start_month) VALUES ('2026-09-01');

-- churches.reporting_start_month is now an override: NULL uses the organization default.
-- A new church starts at the month being filed when it is added (Asia/Manila): added in
-- October, it first reports September. An admin can change it.
CREATE OR REPLACE FUNCTION ensure_church() RETURNS trigger LANGUAGE plpgsql AS $fn$
BEGIN
    IF btrim(COALESCE(NEW.church_name, '')) <> '' THEN
        INSERT INTO churches (name, name_key, reporting_start_month)
        VALUES (btrim(NEW.church_name), lower(btrim(NEW.church_name)),
                (date_trunc('month', now() AT TIME ZONE 'Asia/Manila') - interval '1 month')::date)
        ON CONFLICT (name_key) DO NOTHING;
    END IF;
    RETURN NEW;
END
$fn$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION ensure_church() RETURNS trigger LANGUAGE plpgsql AS $fn$
BEGIN
    IF btrim(COALESCE(NEW.church_name, '')) <> '' THEN
        INSERT INTO churches (name, name_key)
        VALUES (btrim(NEW.church_name), lower(btrim(NEW.church_name)))
        ON CONFLICT (name_key) DO NOTHING;
    END IF;
    RETURN NEW;
END
$fn$;
DROP TABLE IF EXISTS reporting_settings;
-- +goose StatementEnd
