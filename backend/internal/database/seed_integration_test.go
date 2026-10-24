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
	"golang.org/x/crypto/bcrypt"

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
	"reports:export":     "export",
}

// adminAreaPermissions open the review parts of the admin panel (37b).
var adminAreaPermissions = map[string][2]string{
	"reports:export": {"reports", "export"},
	"tracker:read":   {"tracker", "read"},
	"audit:read":     {"audit", "read"},
	"users:list":     {"users", "list"},
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

// withoutAdminOnly drops the report permissions only the admin holds.
func withoutAdminOnly(perms []string) []string {
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		if p != "reports:approve" && p != "reports:export" {
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

	// Only the admin approves and exports.
	assert.ElementsMatch(t,
		append([]string{"users:read", "users:write"}, withoutAdminOnly(allReportPermissions())...),
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

func TestSeed_AdminAreaPermissions(t *testing.T) {
	app := seeded(t)

	for name, want := range adminAreaPermissions {
		var resource, action string
		require.NoError(t, app.QueryRow(context.Background(),
			`SELECT resource, action FROM permissions WHERE name = $1`, name).Scan(&resource, &action), name)
		assert.Equal(t, want, [2]string{resource, action}, name)
	}

	// Only the admin holds them; custom roles get them through the Roles tab.
	for _, role := range []string{"editor", "user"} {
		for _, name := range permissionsOf(t, app, role) {
			_, found := adminAreaPermissions[name]
			assert.False(t, found, "%s holds %s", role, name)
		}
	}
}

func TestSeed_Idempotent(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	app := tdb.AppUserPool(t)
	ctx := context.Background()
	db := &database.DB{Pool: app, Builder: database.NewStatementBuilder()}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	admin := database.AdminSeed{
		Email:    "seed-admin@example.com",
		Name:     "Seed Admin",
		Password: "SeedPassword123!",
	}
	require.NoError(t, database.SeedData(ctx, db, log, admin))

	snapshot := func() map[string][]string {
		return map[string][]string{
			"roles":            names(t, app, `SELECT row_to_json(r)::text FROM roles r ORDER BY id`),
			"permissions":      names(t, app, `SELECT row_to_json(p)::text FROM permissions p ORDER BY id`),
			"role_permissions": names(t, app, `SELECT row_to_json(rp)::text FROM role_permissions rp ORDER BY role_id, permission_id`),
			"users":            names(t, app, `SELECT row_to_json(u)::text FROM users u ORDER BY id`),
		}
	}
	before := snapshot()
	require.Len(t, before["roles"], 3)
	require.Greater(t, len(before["permissions"]), 9)
	require.NotEmpty(t, before["role_permissions"])
	require.Len(t, before["users"], 1)

	var name, hash, role string
	var verified bool
	require.NoError(t, app.QueryRow(ctx, `
		SELECT u.name, u.password_hash, r.name, u.email_verified
		FROM users u JOIN roles r ON r.id = u.role_id
		WHERE u.email = $1`, admin.Email).Scan(&name, &hash, &role, &verified))
	assert.Equal(t, admin.Name, name)
	assert.Equal(t, "admin", role)
	assert.True(t, verified)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte(admin.Password)))

	require.NoError(t, database.SeedData(ctx, db, log, admin))
	assert.Equal(t, before, snapshot(), "reseeding must not duplicate or modify existing rows")
}
