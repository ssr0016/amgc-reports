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

func TestWrites_DuplicateValueIsErrUniqueViolation(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	roles := NewRoleStore(db)
	perms := NewPermissionStore(db)
	users := userimpl.NewStore(db)
	ctx := context.Background()

	editor, err := roles.Create(ctx, accesscontrol.CreateRoleRequest{Name: "editor", Description: "x"})
	require.NoError(t, err)
	viewer, err := roles.Create(ctx, accesscontrol.CreateRoleRequest{Name: "viewer", Description: "x"})
	require.NoError(t, err)
	_, err = perms.Create(ctx, "report:read", "report", "read")
	require.NoError(t, err)
	_, err = users.CreateWithPassword(ctx, "dup@example.com", "Dup", "hash")
	require.NoError(t, err)

	t.Run("create a role with an existing name", func(t *testing.T) {
		_, err := roles.Create(ctx, accesscontrol.CreateRoleRequest{Name: "editor", Description: "y"})
		require.ErrorIs(t, err, database.ErrUniqueViolation)
	})

	t.Run("rename a role to an existing name", func(t *testing.T) {
		_, err := roles.Update(ctx, viewer.ID, accesscontrol.UpdateRoleRequest{Name: "editor"})
		require.ErrorIs(t, err, database.ErrUniqueViolation)
	})

	t.Run("rename a role to its own name is not a violation", func(t *testing.T) {
		got, err := roles.Update(ctx, editor.ID, accesscontrol.UpdateRoleRequest{Name: "editor"})
		require.NoError(t, err)
		require.Equal(t, "editor", got.Name)
	})

	t.Run("create a permission with an existing name", func(t *testing.T) {
		_, err := perms.Create(ctx, "report:read", "report", "read")
		require.ErrorIs(t, err, database.ErrUniqueViolation)
	})

	t.Run("create a user with an existing email", func(t *testing.T) {
		_, err := users.CreateWithPassword(ctx, "dup@example.com", "Dup 2", "hash")
		require.ErrorIs(t, err, database.ErrUniqueViolation)
	})
}
