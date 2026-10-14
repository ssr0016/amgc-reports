//go:build integration

package database_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

// reportPermissions maps each D-1 permission to its action.
var reportPermissions = map[string]string{
	"reports:read":       "read",
	"reports:read_any":   "read_any",
	"reports:write":      "write",
	"reports:write_any":  "write_any",
	"reports:delete":     "delete",
	"reports:delete_any": "delete_any",
	"reports:approve":    "approve",
}

// seeded migrates, then seeds as app_user: the role the server uses.
func seeded(t *testing.T) *pgxpool.Pool {
	t.Helper()
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	app := tdb.AppUserPool(t)
	seedAs(t, app)
	return app
}

func seedAs(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	db := &database.DB{Pool: pool, Builder: database.NewStatementBuilder()}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	require.NoError(t, database.SeedData(context.Background(), db, log, database.AdminSeed{}))
}

func names(t *testing.T, pool *pgxpool.Pool, query string, args ...any) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), query, args...)
	require.NoError(t, err)
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		out = append(out, name)
	}
	require.NoError(t, rows.Err())
	return out
}

func permissionsOf(t *testing.T, pool *pgxpool.Pool, role string) []string {
	t.Helper()
	return names(t, pool,
		`SELECT p.name FROM role_permissions rp
		 JOIN roles r ON r.id = rp.role_id
		 JOIN permissions p ON p.id = rp.permission_id
		 WHERE r.name = $1 ORDER BY p.name`, role)
}

func allReportPermissions() []string {
	out := make([]string, 0, len(reportPermissions))
	for name := range reportPermissions {
		out = append(out, name)
	}
	return out
}

func withoutApprove(perms []string) []string {
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		if p != "reports:approve" {
			out = append(out, p)
		}
	}
	return out
}

func TestSeed_ReportsPermissionsExist(t *testing.T) {
	app := seeded(t)

	rows, err := app.Query(context.Background(),
		`SELECT name, resource, action FROM permissions WHERE resource = 'reports'`)
	require.NoError(t, err)
	defer rows.Close()

	got := map[string]string{}
	for rows.Next() {
		var name, resource, action string
		require.NoError(t, rows.Scan(&name, &resource, &action))
		assert.Equal(t, "reports", resource)
		got[name] = action
	}
	require.NoError(t, rows.Err())

	assert.Equal(t, reportPermissions, got)
}

func TestSeed_RoleDefaults(t *testing.T) {
	app := seeded(t)

	assert.ElementsMatch(t,
		[]string{"users:read", "reports:read", "reports:read_any", "reports:write", "reports:delete"},
		permissionsOf(t, app, "user"))

	// Only the admin approves.
	assert.ElementsMatch(t,
		append([]string{"users:read", "users:write"}, withoutApprove(allReportPermissions())...),
		permissionsOf(t, app, "editor"))

	assert.ElementsMatch(t,
		names(t, app, `SELECT name FROM permissions`),
		permissionsOf(t, app, "admin"), "admin must hold every permission")
	assert.Subset(t, permissionsOf(t, app, "admin"), allReportPermissions())
}

// Every signed-in user reads every report (read-only); only the owner, an editor or
// an admin may change one, and only the admin approves.
func TestSeed_NoPrivilegeLeakToUser(t *testing.T) {
	app := seeded(t)

	for _, name := range permissionsOf(t, app, "user") {
		if name == "reports:read_any" {
			continue
		}
		assert.False(t, strings.HasSuffix(name, "_any"), "user holds %s", name)
		assert.NotEqual(t, "reports:approve", name, "user must not approve reports")
	}
}

func TestSeed_OnlyAdminApproves(t *testing.T) {
	app := seeded(t)

	roles := names(t, app, `SELECT r.name FROM role_permissions rp
		JOIN roles r ON r.id = rp.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE p.name = 'reports:approve'`)
	assert.Equal(t, []string{"admin"}, roles)
}

func TestSeed_IdempotentWithReports(t *testing.T) {
	app := seeded(t)
	ctx := context.Background()
	count := func(table string) (n int) {
		// table is a fixed identifier from this test.
		query := map[string]string{
			"permissions":      `SELECT COUNT(*) FROM permissions`,
			"role_permissions": `SELECT COUNT(*) FROM role_permissions`,
		}[table]
		require.NoError(t, app.QueryRow(ctx, query).Scan(&n))
		return n
	}
	permsBefore, assignBefore := count("permissions"), count("role_permissions")
	require.Greater(t, permsBefore, 9, "the Reports permissions were not seeded")

	seedAs(t, app)

	assert.Equal(t, permsBefore, count("permissions"))
	assert.Equal(t, assignBefore, count("role_permissions"))
}
