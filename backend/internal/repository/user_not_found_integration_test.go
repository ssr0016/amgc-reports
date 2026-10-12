//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

func TestUserWrites_MissingUserIsErrNotFound(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	roles := NewRoleRepo(db)
	users := NewUserRepo(db)
	ctx := context.Background()

	role, err := roles.Create(ctx, model.CreateRoleRequest{Name: "nf-role", Description: "x"})
	require.NoError(t, err)

	// There is no user-delete API; remove the row directly to model a user
	// that vanishes between a handler's check and its write.
	gone, err := users.CreateWithPassword(ctx, "gone@example.com", "Gone", "hash")
	require.NoError(t, err)
	_, err = tdb.Pool.Exec(ctx, "DELETE FROM users WHERE id = $1", gone.ID)
	require.NoError(t, err)

	t.Run("UpdateRole", func(t *testing.T) {
		require.ErrorIs(t, users.UpdateRole(ctx, gone.ID, role.ID), ErrNotFound)
	})

	t.Run("MarkEmailVerified", func(t *testing.T) {
		require.ErrorIs(t, users.MarkEmailVerified(ctx, gone.ID), ErrNotFound)
	})

	t.Run("UpdatePassword", func(t *testing.T) {
		require.ErrorIs(t, users.UpdatePassword(ctx, gone.ID, "newhash"), ErrNotFound)
	})

	t.Run("RecordFailedLogin", func(t *testing.T) {
		require.ErrorIs(t, users.RecordFailedLogin(ctx, gone.ID, 5, 15*time.Minute), ErrNotFound)
	})

	t.Run("ResetLoginAttempts", func(t *testing.T) {
		require.ErrorIs(t, users.ResetLoginAttempts(ctx, gone.ID), ErrNotFound)
	})

	t.Run("UpdateRole with a missing role stays a foreign key violation", func(t *testing.T) {
		present, err := users.CreateWithPassword(ctx, "present@example.com", "Present", "hash")
		require.NoError(t, err)

		err = users.UpdateRole(ctx, present.ID, 99999)
		require.ErrorIs(t, err, ErrForeignKeyViolation)
		require.False(t, errors.Is(err, ErrNotFound), "a missing role must not read as a missing user: %v", err)
	})
}
