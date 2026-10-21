//go:build integration

package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

// Production wiring: the server connects as app_user and the pre-deploy
// command as migrate_user. With nothing pending, the run must succeed and
// leave the applied versions as they were.
func TestMigrateUser_CanRunMigrateCommand(t *testing.T) {
	testutil.SkipMigrateUser(t)
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	before := appliedVersions(t, tdb)

	t.Setenv("DATABASE_URL", tdb.RoleDSN(t, "app_user"))
	t.Setenv("MIGRATE_DATABASE_URL", tdb.RoleDSN(t, "migrate_user"))

	var buf bytes.Buffer
	code := dispatch([]string{"migrate"}, &buf, commands{
		run:     func() error { t.Fatal("migrate started the server"); return nil },
		migrate: runMigrate,
	})

	require.Equal(t, 0, code, "stderr: %s", buf.String())
	assert.Equal(t, before, appliedVersions(t, tdb))
	assert.Equal(t, embeddedMigrationCount(t), appliedVersions(t, tdb))
}

// The role is logged so a deploy shows which credentials ran the migrations.
func TestCurrentRole_ReportsConnectedRole(t *testing.T) {
	testutil.SkipMigrateUser(t)
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)

	got, err := currentRole(t.Context(), tdb.RolePool(t, "migrate_user"))

	require.NoError(t, err)
	assert.Equal(t, "migrate_user", got)
}
