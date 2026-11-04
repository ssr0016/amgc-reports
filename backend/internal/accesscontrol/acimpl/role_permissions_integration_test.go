//go:build integration

package acimpl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

func TestRoleStore_GetPermissionsByRoleIDs_Postgres(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	roles := NewRoleStore(db)
	permissions := NewPermissionStore(db)
	ctx := context.Background()

	read, err := permissions.Create(ctx, "reports:read_any", "reports", "read_any")
	require.NoError(t, err)
	approve, err := permissions.Create(ctx, "reports:approve", "reports", "approve")
	require.NoError(t, err)
	reviewer, err := roles.Create(ctx, accesscontrol.CreateRoleRequest{Name: "Reviewer"})
	require.NoError(t, err)
	reader, err := roles.Create(ctx, accesscontrol.CreateRoleRequest{Name: "Reader"})
	require.NoError(t, err)
	empty, err := roles.Create(ctx, accesscontrol.CreateRoleRequest{Name: "No grants"})
	require.NoError(t, err)
	excluded, err := roles.Create(ctx, accesscontrol.CreateRoleRequest{Name: "Excluded"})
	require.NoError(t, err)
	require.NoError(t, roles.ReplacePermissions(ctx, reviewer.ID, []int64{read.ID, approve.ID}))
	require.NoError(t, roles.AssignPermission(ctx, reader.ID, read.ID))
	require.NoError(t, roles.AssignPermission(ctx, excluded.ID, approve.ID))

	got, err := roles.GetPermissionsByRoleIDs(ctx, []int64{reader.ID, empty.ID, reviewer.ID, 99999})
	require.NoError(t, err)
	assert.Equal(t, map[int64][]accesscontrol.Permission{
		reviewer.ID: {*approve, *read},
		reader.ID:   {*read},
	}, got, "group shared permissions, keep name order, and exclude roles outside the page")

	single, err := roles.GetPermissions(ctx, reviewer.ID)
	require.NoError(t, err)
	assert.Equal(t, []accesscontrol.Permission{*approve, *read}, single)

	for _, ids := range [][]int64{nil, {}} {
		got, err := roles.GetPermissionsByRoleIDs(ctx, ids)
		require.NoError(t, err)
		assert.Empty(t, got)
	}
}
