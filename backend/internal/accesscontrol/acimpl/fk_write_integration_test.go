//go:build integration

package acimpl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

func TestWrites_MissingParentIsErrForeignKeyViolation(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	roles := NewRoleStore(db)
	perms := NewPermissionStore(db)
	users := userimpl.NewStore(db)
	ctx := context.Background()

	role, err := roles.Create(ctx, accesscontrol.CreateRoleRequest{Name: "fk-role", Description: "x"})
	require.NoError(t, err)
	perm, err := perms.Create(ctx, "fk:perm", "fk", "perm")
	require.NoError(t, err)
	user, err := users.CreateWithPassword(ctx, "fk@example.com", "FK", "hash")
	require.NoError(t, err)

	t.Run("assign to a missing role", func(t *testing.T) {
		require.ErrorIs(t, roles.AssignPermission(ctx, 99999, perm.ID), database.ErrForeignKeyViolation)
	})

	t.Run("assign a missing permission", func(t *testing.T) {
		require.ErrorIs(t, roles.AssignPermission(ctx, role.ID, 99999), database.ErrForeignKeyViolation)
	})

	t.Run("change a user to a missing role", func(t *testing.T) {
		require.ErrorIs(t, users.UpdateRole(ctx, user.ID, 99999), database.ErrForeignKeyViolation)
	})
}
