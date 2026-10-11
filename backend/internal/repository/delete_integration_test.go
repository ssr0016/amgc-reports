//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

func TestDelete_MissingRowIsErrNotFound(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	roles := NewRoleRepo(db)
	perms := NewPermissionRepo(db)
	users := NewUserRepo(db)
	ctx := context.Background()

	t.Run("missing role", func(t *testing.T) {
		err := roles.Delete(ctx, 99999)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNotFound)
		require.Contains(t, err.Error(), "roles", "wrapped error should name the table")
	})

	t.Run("missing permission", func(t *testing.T) {
		err := perms.Delete(ctx, 99999)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNotFound)
		require.Contains(t, err.Error(), "permissions")
	})

	t.Run("existing role deletes once, then is not found", func(t *testing.T) {
		role, err := roles.Create(ctx, model.CreateRoleRequest{Name: "temp-role", Description: "x"})
		require.NoError(t, err)

		require.NoError(t, roles.Delete(ctx, role.ID))

		err = roles.Delete(ctx, role.ID)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("existing permission deletes once, then is not found", func(t *testing.T) {
		perm, err := perms.Create(ctx, "temp:perm", "temp", "perm")
		require.NoError(t, err)

		require.NoError(t, perms.Delete(ctx, perm.ID))

		err = perms.Delete(ctx, perm.ID)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("role still assigned to a user is not ErrNotFound", func(t *testing.T) {
		role, err := roles.Create(ctx, model.CreateRoleRequest{Name: "in-use", Description: "x"})
		require.NoError(t, err)
		user, err := users.CreateWithPassword(ctx, "inuse@example.com", "In Use", "hash")
		require.NoError(t, err)
		require.NoError(t, users.UpdateRole(ctx, user.ID, role.ID))

		err = roles.Delete(ctx, role.ID)
		require.Error(t, err)
		require.False(t, errors.Is(err, ErrNotFound), "FK violation must stay distinct from not-found: %v", err)
	})
}
