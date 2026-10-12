-- +goose Up
-- +goose StatementBegin
-- app_user is the role the server connects as: DML only, no DDL. The migrator
-- (MIGRATE_DATABASE_URL) keeps ownership. The role is NOLOGIN and has no
-- password here; production enables it out-of-band
-- (ALTER ROLE app_user LOGIN PASSWORD ...) before DATABASE_URL points at it.
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'app_user') THEN
        CREATE ROLE app_user NOLOGIN;
    END IF;

    -- Database name differs between prod, dev and test containers.
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO app_user', current_database());
END
$$;

GRANT USAGE ON SCHEMA public TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO app_user;

-- Applies to objects created later by the role running this migration.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO app_user;

-- Migration state is not the server's to read or change.
REVOKE ALL ON goose_db_version FROM app_user, PUBLIC;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Deliberately a no-op: roles are cluster-scoped, and a rollback must not drop
-- a role that production has started using. Drop it by hand if you mean to.
SELECT 1;
-- +goose StatementEnd
