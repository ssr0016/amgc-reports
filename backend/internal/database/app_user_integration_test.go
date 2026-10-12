//go:build integration

package database_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

const insufficientPrivilege = "42501"

// exerciseDML runs the statements the server issues, as the given pool, and
// fails the test on the first one that is refused.
func exerciseDML(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	var roleID, userID int64
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO roles (name) VALUES ('probe') RETURNING id`).Scan(&roleID), "insert into roles (needs sequence usage)")
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email, name, password_hash, role_id) VALUES ('p@example.com', 'P', 'x', $1) RETURNING id`,
		roleID).Scan(&userID), "insert into users")
	_, err := pool.Exec(ctx, `UPDATE users SET name = 'Q' WHERE id = $1`, userID)
	require.NoError(t, err, "update users")
	var name string
	require.NoError(t, pool.QueryRow(ctx, `SELECT name FROM users WHERE id = $1`, userID).Scan(&name), "select users")
	_, err = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	require.NoError(t, err, "delete users")
	_, err = pool.Exec(ctx, `DELETE FROM roles WHERE id = $1`, roleID)
	require.NoError(t, err, "delete roles")
}

func TestAppUser_CanRunApplicationDML(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)

	exerciseDML(t, tdb.AppUserPool(t))
}

func TestAppUser_CannotChangeSchemaOrTouchMigrationState(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	app := tdb.AppUserPool(t)

	tests := []struct{ name, sql string }{
		{"create table", `CREATE TABLE evil (id int)`},
		{"drop table", `DROP TABLE users`},
		{"alter table", `ALTER TABLE users ADD COLUMN evil int`},
		{"truncate", `TRUNCATE users CASCADE`},
		{"read goose_db_version", `SELECT * FROM goose_db_version`},
		{"write goose_db_version", `DELETE FROM goose_db_version`},
		{"create role", `CREATE ROLE evil`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := app.Exec(context.Background(), tt.sql)

			var pgErr *pgconn.PgError
			require.True(t, errors.As(err, &pgErr), "want a Postgres error, got %v", err)
			assert.Equal(t, insufficientPrivilege, pgErr.Code, pgErr.Message)
		})
	}
}

// Tables a later migration creates must be usable without another grant.
func TestAppUser_GetsPrivilegesOnTablesCreatedLater(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	ctx := context.Background()

	_, err := tdb.Pool.Exec(ctx, `CREATE TABLE later (id BIGSERIAL PRIMARY KEY, v TEXT)`)
	require.NoError(t, err)

	_, err = tdb.AppUserPool(t).Exec(ctx, `INSERT INTO later (v) VALUES ('x')`)
	assert.NoError(t, err, "default privileges did not cover a new table and its sequence")
}

// A role created by hand beforehand (or by a previous failed rollout) must not
// make the migration fail.
func TestAppUser_MigrationAdoptsExistingRole(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	_, err := tdb.Pool.Exec(context.Background(), `CREATE ROLE app_user NOLOGIN`)
	require.NoError(t, err)

	tdb.MigrateWithGoose(t)

	exerciseDML(t, tdb.AppUserPool(t))
}

// Roles are cluster-scoped; rolling back must not drop one production uses.
func TestAppUser_DownMigrationKeepsRole_AndUpAgainSucceeds(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)

	db := sql.OpenDB(stdlib.GetPoolConnector(tdb.Pool))
	defer func() { _ = db.Close() }()
	goose.SetBaseFS(os.DirFS("migrations"))
	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.Down(db, "."), "down migration failed")

	var n int
	require.NoError(t, tdb.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM pg_roles WHERE rolname = 'app_user'`).Scan(&n))
	assert.Equal(t, 1, n, "down dropped app_user")

	tdb.MigrateWithGoose(t)
	exerciseDML(t, tdb.AppUserPool(t))
}

func TestAppUser_IsNoLoginUntilEnabled(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)

	var canLogin bool
	require.NoError(t, tdb.Pool.QueryRow(context.Background(),
		`SELECT rolcanlogin FROM pg_roles WHERE rolname = 'app_user'`).Scan(&canLogin))
	assert.False(t, canLogin, "app_user must not be able to log in until production enables it")
}
