//go:build integration

package main

import (
	"bytes"
	"context"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

// migrateWith runs `api migrate` through dispatch against url and returns the
// exit code. run must never be called by the migrate subcommand.
func migrateWith(t *testing.T, url string) (code int, stderr string) {
	t.Helper()
	t.Setenv("DATABASE_URL", url)
	t.Setenv("MIGRATE_DATABASE_URL", "")

	var buf bytes.Buffer
	code = dispatch([]string{"migrate"}, &buf, commands{
		run:     func() error { t.Fatal("migrate started the server"); return nil },
		migrate: runMigrate,
	})
	return code, buf.String()
}

func embeddedMigrationCount(t *testing.T) int {
	t.Helper()
	files, err := filepath.Glob("../../internal/database/migrations/*.sql")
	require.NoError(t, err)
	require.NotEmpty(t, files, "no migration files found")
	return len(files)
}

func appliedVersions(t *testing.T, tdb *testutil.TestDB) int {
	t.Helper()
	var n int
	err := tdb.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM goose_db_version WHERE version_id > 0 AND is_applied`).Scan(&n)
	require.NoError(t, err)
	return n
}

func tableExists(t *testing.T, tdb *testutil.TestDB, name string) bool {
	t.Helper()
	var exists bool
	err := tdb.Pool.QueryRow(context.Background(),
		`SELECT to_regclass($1) IS NOT NULL`, "public."+name).Scan(&exists)
	require.NoError(t, err)
	return exists
}

func TestRunMigrate_FreshDB_AppliesAllMigrations(t *testing.T) {
	tdb := testutil.SetupPostgres(t)

	code, stderr := migrateWith(t, tdb.DSN)

	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, embeddedMigrationCount(t), appliedVersions(t, tdb))
	for _, table := range []string{
		"users", "sessions", "roles", "permissions", "role_permissions",
		"verification_tokens", "password_resets",
	} {
		assert.True(t, tableExists(t, tdb, table), "table %s was not created", table)
	}
}

func TestRunMigrate_SecondRun_IsIdempotent(t *testing.T) {
	tdb := testutil.SetupPostgres(t)

	code, stderr := migrateWith(t, tdb.DSN)
	require.Equal(t, 0, code, "first run, stderr: %s", stderr)
	first := appliedVersions(t, tdb)

	code, stderr = migrateWith(t, tdb.DSN)
	require.Equal(t, 0, code, "second run, stderr: %s", stderr)

	assert.Equal(t, first, appliedVersions(t, tdb), "a second run changed the applied versions")
	assert.Equal(t, embeddedMigrationCount(t), appliedVersions(t, tdb))
}

// The pre-deploy step must connect as the migration owner, not as the app role.
func TestRunMigrate_UsesMigrateDatabaseURL_WhenSet(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	t.Setenv("DATABASE_URL", "postgres://app:wrong@127.0.0.1:1/db?sslmode=disable&connect_timeout=2")
	t.Setenv("MIGRATE_DATABASE_URL", tdb.DSN)

	var buf bytes.Buffer
	code := dispatch([]string{"migrate"}, &buf, commands{
		run:     func() error { t.Fatal("migrate started the server"); return nil },
		migrate: runMigrate,
	})

	require.Equal(t, 0, code, "stderr: %s", buf.String())
	assert.Equal(t, embeddedMigrationCount(t), appliedVersions(t, tdb))
}

func TestRunMigrate_BadURL_ExitsNonZero_NoPartialState(t *testing.T) {
	tdb := testutil.SetupPostgres(t)

	// A real server and credentials, but a database that does not exist, and a
	// URL that is not a URL at all.
	missingDB := replaceDatabase(t, tdb.DSN, "no_such_db")

	tests := []struct {
		name string
		url  string
	}{
		{"database does not exist", missingDB},
		{"unparseable URL", "not a url"},
		{"unreachable host", "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stderr := migrateWith(t, tt.url)

			assert.Equal(t, 1, code, "stderr: %s", stderr)
			assert.False(t, tableExists(t, tdb, "goose_db_version"),
				"a failed run left migration state in the real database")
			assert.False(t, tableExists(t, tdb, "users"),
				"a failed run left tables in the real database")
		})
	}

	var n int
	require.NoError(t, tdb.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM pg_database WHERE datname = 'no_such_db'`).Scan(&n))
	assert.Zero(t, n, "a failed run created a database")
}

func replaceDatabase(t *testing.T, dsn, name string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.Path = "/" + name
	return u.String()
}
