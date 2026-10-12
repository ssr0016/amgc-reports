//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

// Register assigns the default role, so deleting or renaming it would break
// registration until the next restart re-seeds it.
func TestDefaultRole_IsProtected(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	roles := NewRoleRepo(db)
	ctx := context.Background()

	def, err := roles.GetByName(ctx, model.DefaultRoleName)
	require.NoError(t, err)
	require.NotNil(t, def, "default role must be seeded")

	t.Run("delete is refused even when no user holds it", func(t *testing.T) {
		require.ErrorIs(t, roles.Delete(ctx, def.ID), ErrProtectedRole)

		still, err := roles.GetByID(ctx, def.ID)
		require.NoError(t, err)
		require.NotNil(t, still, "a refused delete must leave the role in place")
	})

	t.Run("rename is refused", func(t *testing.T) {
		got, err := roles.Update(ctx, def.ID, model.UpdateRoleRequest{Name: "member"})
		require.ErrorIs(t, err, ErrProtectedRole)
		require.Nil(t, got)

		still, err := roles.GetByID(ctx, def.ID)
		require.NoError(t, err)
		require.Equal(t, model.DefaultRoleName, still.Name)
	})

	t.Run("description-only update still works", func(t *testing.T) {
		got, err := roles.Update(ctx, def.ID, model.UpdateRoleRequest{Description: "new description"})
		require.NoError(t, err)
		require.Equal(t, "new description", got.Description)
		require.Equal(t, model.DefaultRoleName, got.Name)
	})

	t.Run("update that keeps the name still works", func(t *testing.T) {
		got, err := roles.Update(ctx, def.ID, model.UpdateRoleRequest{Name: model.DefaultRoleName, Description: "same name"})
		require.NoError(t, err)
		require.Equal(t, "same name", got.Description)
	})

	t.Run("other roles are unaffected", func(t *testing.T) {
		other, err := roles.Create(ctx, model.CreateRoleRequest{Name: "temp-role", Description: "x"})
		require.NoError(t, err)

		renamed, err := roles.Update(ctx, other.ID, model.UpdateRoleRequest{Name: "temp-role-2"})
		require.NoError(t, err)
		require.Equal(t, "temp-role-2", renamed.Name)
		require.NoError(t, roles.Delete(ctx, other.ID))
	})
}

func TestCreateWithPassword_MissingDefaultRole(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := NewUserRepo(db)

	tdb.RemoveRole(t, model.DefaultRoleName)

	u, err := users.CreateWithPassword(context.Background(), "norole@example.com", "No Role", "hash")
	require.ErrorIs(t, err, ErrDefaultRoleMissing)
	require.Nil(t, u)
}
