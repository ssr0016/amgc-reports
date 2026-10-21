//go:build integration

package userimpl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol/acimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

// Register assigns the default role, so deleting or renaming it would break
// registration until the next restart re-seeds it.
func TestDefaultRole_IsProtected(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	roles := acimpl.NewRoleStore(db)
	ctx := context.Background()

	def, err := roles.GetByName(ctx, accesscontrol.DefaultRoleName)
	require.NoError(t, err)
	require.NotNil(t, def, "default role must be seeded")

	t.Run("delete is refused even when no user holds it", func(t *testing.T) {
		require.ErrorIs(t, roles.Delete(ctx, def.ID), accesscontrol.ErrProtectedRole)

		still, err := roles.GetByID(ctx, def.ID)
		require.NoError(t, err)
		require.NotNil(t, still, "a refused delete must leave the role in place")
	})

	t.Run("rename is refused", func(t *testing.T) {
		got, err := roles.Update(ctx, def.ID, accesscontrol.UpdateRoleRequest{Name: "member"})
		require.ErrorIs(t, err, accesscontrol.ErrProtectedRole)
		require.Nil(t, got)

		still, err := roles.GetByID(ctx, def.ID)
		require.NoError(t, err)
		require.Equal(t, accesscontrol.DefaultRoleName, still.Name)
	})

	t.Run("description-only update still works", func(t *testing.T) {
		got, err := roles.Update(ctx, def.ID, accesscontrol.UpdateRoleRequest{Description: "new description"})
		require.NoError(t, err)
		require.Equal(t, "new description", got.Description)
		require.Equal(t, accesscontrol.DefaultRoleName, got.Name)
	})

	t.Run("update that keeps the name still works", func(t *testing.T) {
		got, err := roles.Update(ctx, def.ID, accesscontrol.UpdateRoleRequest{Name: accesscontrol.DefaultRoleName, Description: "same name"})
		require.NoError(t, err)
		require.Equal(t, "same name", got.Description)
	})

	t.Run("other roles are unaffected", func(t *testing.T) {
		other, err := roles.Create(ctx, accesscontrol.CreateRoleRequest{Name: "temp-role", Description: "x"})
		require.NoError(t, err)

		renamed, err := roles.Update(ctx, other.ID, accesscontrol.UpdateRoleRequest{Name: "temp-role-2"})
		require.NoError(t, err)
		require.Equal(t, "temp-role-2", renamed.Name)
		require.NoError(t, roles.Delete(ctx, other.ID))
	})
}

func TestCreateWithPassword_MissingDefaultRole(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := NewStore(db)

	tdb.RemoveRole(t, accesscontrol.DefaultRoleName)

	u, err := users.CreateWithPassword(context.Background(), "norole@example.com", "No Role", "hash")
	require.ErrorIs(t, err, accesscontrol.ErrDefaultRoleMissing)
	require.Nil(t, u)
}
