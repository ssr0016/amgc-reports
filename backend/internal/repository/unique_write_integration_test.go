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

func TestWrites_DuplicateValueIsErrUniqueViolation(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	roles := NewRoleRepo(db)
	perms := NewPermissionRepo(db)
	users := NewUserRepo(db)
	ctx := context.Background()

	editor, err := roles.Create(ctx, model.CreateRoleRequest{Name: "editor", Description: "x"})
	require.NoError(t, err)
	viewer, err := roles.Create(ctx, model.CreateRoleRequest{Name: "viewer", Description: "x"})
	require.NoError(t, err)
	_, err = perms.Create(ctx, "report:read", "report", "read")
	require.NoError(t, err)
	_, err = users.CreateWithPassword(ctx, "dup@example.com", "Dup", "hash")
	require.NoError(t, err)

	t.Run("create a role with an existing name", func(t *testing.T) {
		_, err := roles.Create(ctx, model.CreateRoleRequest{Name: "editor", Description: "y"})
		require.ErrorIs(t, err, ErrUniqueViolation)
	})

	t.Run("rename a role to an existing name", func(t *testing.T) {
		_, err := roles.Update(ctx, viewer.ID, model.UpdateRoleRequest{Name: "editor"})
		require.ErrorIs(t, err, ErrUniqueViolation)
	})

	t.Run("rename a role to its own name is not a violation", func(t *testing.T) {
		got, err := roles.Update(ctx, editor.ID, model.UpdateRoleRequest{Name: "editor"})
		require.NoError(t, err)
		require.Equal(t, "editor", got.Name)
	})

	t.Run("create a permission with an existing name", func(t *testing.T) {
		_, err := perms.Create(ctx, "report:read", "report", "read")
		require.ErrorIs(t, err, ErrUniqueViolation)
	})

	t.Run("create a user with an existing email", func(t *testing.T) {
		_, err := users.CreateWithPassword(ctx, "dup@example.com", "Dup 2", "hash")
		require.ErrorIs(t, err, ErrUniqueViolation)
	})
}
