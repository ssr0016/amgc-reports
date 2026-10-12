//go:build integration

package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

// migrate_user owns the schema's objects, because DDL needs ownership; without
// that its first ALTER TABLE fails with "must be owner".
func TestMigrateUser_OwnsSchemaObjects(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)

	rows, err := tdb.Pool.Query(context.Background(),
		`SELECT tablename, tableowner FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`)
	require.NoError(t, err)
	defer rows.Close()

	owners := map[string]string{}
	for rows.Next() {
		var table, owner string
		require.NoError(t, rows.Scan(&table, &owner))
		owners[table] = owner
	}
	require.NoError(t, rows.Err())

	require.Contains(t, owners, "goose_db_version")
	require.Contains(t, owners, "users")
	for table, owner := range owners {
		assert.Equal(t, "migrate_user", owner, "table %s is owned by %s", table, owner)
	}
}

func TestMigrateUser_CanAlterExistingTable(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	mig := tdb.RolePool(t, "migrate_user")

	_, err := mig.Exec(context.Background(), `ALTER TABLE users ADD COLUMN probe TEXT`)

	assert.NoError(t, err)
}

// A table migrate_user creates must be usable by app_user without another
// grant: the earlier default privileges were set for the role that ran that
// migration, not for migrate_user. Migration state stays off limits.
func TestMigrateUser_NewTablesAreUsableByAppUser(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	ctx := context.Background()
	mig := tdb.RolePool(t, "migrate_user")
	app := tdb.AppUserPool(t)

	_, err := mig.Exec(ctx, `CREATE TABLE later_mu (id BIGSERIAL PRIMARY KEY, v TEXT)`)
	require.NoError(t, err)

	var id int64
	require.NoError(t, app.QueryRow(ctx, `INSERT INTO later_mu (v) VALUES ('x') RETURNING id`).Scan(&id),
		"app_user cannot write a table migrate_user created (default privileges or sequence)")
	var v string
	require.NoError(t, app.QueryRow(ctx, `SELECT v FROM later_mu WHERE id = $1`, id).Scan(&v))

	_, err = app.Exec(ctx, `SELECT * FROM goose_db_version`)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "want a Postgres error, got %v", err)
	assert.Equal(t, insufficientPrivilege, pgErr.Code, "app_user can read goose_db_version")
}

// migrate_user owns the tables, so it can still drop them. What it must not be
// is a superuser: no role creation, no server configuration, no program exec.
func TestMigrateUser_CannotEscalate(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	ctx := context.Background()
	mig := tdb.RolePool(t, "migrate_user")

	var super, createRole, createDB bool
	require.NoError(t, tdb.Pool.QueryRow(ctx,
		`SELECT rolsuper, rolcreaterole, rolcreatedb FROM pg_roles WHERE rolname = 'migrate_user'`).
		Scan(&super, &createRole, &createDB))
	assert.False(t, super, "migrate_user is a superuser")
	assert.False(t, createRole, "migrate_user can create roles")
	assert.False(t, createDB, "migrate_user can create databases")

	tests := []struct{ name, sql string }{
		{"create role", `CREATE ROLE evil`},
		{"alter system", `ALTER SYSTEM SET work_mem = '8MB'`},
		{"copy to program", `COPY (SELECT 1) TO PROGRAM 'true'`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mig.Exec(ctx, tt.sql)

			var pgErr *pgconn.PgError
			require.True(t, errors.As(err, &pgErr), "want a Postgres error, got %v", err)
			assert.Equal(t, insufficientPrivilege, pgErr.Code, pgErr.Message)
		})
	}
}
