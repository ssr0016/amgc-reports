-- +goose Up
-- +goose StatementBegin
-- migrate_user is the role `api migrate` connects as (MIGRATE_DATABASE_URL). It
-- owns the schema's objects, because DDL needs ownership, but it is not a
-- superuser: it cannot create roles, change server settings or run programs.
-- It can still DROP the tables it owns. The role is NOLOGIN and has no password
-- here; production enables it out-of-band
-- (ALTER ROLE migrate_user LOGIN PASSWORD ...) before MIGRATE_DATABASE_URL
-- points at it.
--
-- This migration must run as a superuser or the database owner (it creates a
-- role and changes ownership). A fresh environment is bootstrapped that way.
DO $$
DECLARE
    r record;
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'migrate_user') THEN
        CREATE ROLE migrate_user NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE;
    END IF;

    -- Database name differs between prod, dev and test containers.
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO migrate_user', current_database());

    -- Sequences owned by serial columns change owner with their table.
    FOR r IN SELECT format('%I.%I', schemaname, tablename) AS name
             FROM pg_tables WHERE schemaname = 'public'
    LOOP
        EXECUTE format('ALTER TABLE %s OWNER TO migrate_user', r.name);
    END LOOP;
END
$$;

GRANT USAGE, CREATE ON SCHEMA public TO migrate_user;

-- Tables migrate_user creates later must stay usable by app_user. The defaults
-- set by the app_user migration apply only to objects created by the role that
-- ran it, so they are repeated here FOR migrate_user.
ALTER DEFAULT PRIVILEGES FOR ROLE migrate_user IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_user;
ALTER DEFAULT PRIVILEGES FOR ROLE migrate_user IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO app_user;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Deliberately a no-op: roles are cluster-scoped, and a rollback must not drop
-- a role that production has started using. Drop it by hand if you mean to.
SELECT 1;
-- +goose StatementEnd
