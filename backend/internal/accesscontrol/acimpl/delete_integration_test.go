//go:build integration

package acimpl

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

func TestDelete_MissingRowIsErrNotFound(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	roles := NewRoleStore(db)
	perms := NewPermissionStore(db)
	users := userimpl.NewStore(db)
	ctx := context.Background()

	t.Run("missing role", func(t *testing.T) {
		err := roles.Delete(ctx, 99999)
		require.Error(t, err)
		require.ErrorIs(t, err, database.ErrNotFound)
		require.Contains(t, err.Error(), "roles", "wrapped error should name the table")
	})

	t.Run("missing permission", func(t *testing.T) {
		err := perms.Delete(ctx, 99999)
		require.Error(t, err)
		require.ErrorIs(t, err, database.ErrNotFound)
		require.Contains(t, err.Error(), "permissions")
	})

	t.Run("existing role deletes once, then is not found", func(t *testing.T) {
		role, err := roles.Create(ctx, accesscontrol.CreateRoleRequest{Name: "temp-role", Description: "x"})
		require.NoError(t, err)

		require.NoError(t, roles.Delete(ctx, role.ID))

		err = roles.Delete(ctx, role.ID)
		require.ErrorIs(t, err, database.ErrNotFound)
	})

	t.Run("existing permission deletes once, then is not found", func(t *testing.T) {
		perm, err := perms.Create(ctx, "temp:perm", "temp", "perm")
		require.NoError(t, err)

		require.NoError(t, perms.Delete(ctx, perm.ID))

		err = perms.Delete(ctx, perm.ID)
		require.ErrorIs(t, err, database.ErrNotFound)
	})

	t.Run("role still assigned to a user is database.ErrForeignKeyViolation", func(t *testing.T) {
		role, err := roles.Create(ctx, accesscontrol.CreateRoleRequest{Name: "in-use", Description: "x"})
		require.NoError(t, err)
		user, err := users.CreateWithPassword(ctx, "inuse@example.com", "In Use", "hash")
		require.NoError(t, err)
		require.NoError(t, users.UpdateRole(ctx, user.ID, role.ID))

		err = roles.Delete(ctx, role.ID)
		require.ErrorIs(t, err, database.ErrForeignKeyViolation)
		require.False(t, errors.Is(err, database.ErrNotFound), "FK violation must stay distinct from not-found: %v", err)

		still, err := roles.GetByID(ctx, role.ID)
		require.NoError(t, err)
		require.NotNil(t, still, "a refused delete must leave the role in place")
	})

	t.Run("permission assigned to a role deletes (ON DELETE CASCADE)", func(t *testing.T) {
		role, err := roles.Create(ctx, accesscontrol.CreateRoleRequest{Name: "has-perm", Description: "x"})
		require.NoError(t, err)
		perm, err := perms.Create(ctx, "cascade:perm", "cascade", "perm")
		require.NoError(t, err)
		require.NoError(t, roles.AssignPermission(ctx, role.ID, perm.ID))

		require.NoError(t, perms.Delete(ctx, perm.ID))

		assigned, err := roles.GetPermissions(ctx, role.ID)
		require.NoError(t, err)
		require.Empty(t, assigned)
	})
}
