//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lockOut(t *testing.T, users *UserRepo, id int64) {
	t.Helper()
	for range 3 {
		require.NoError(t, users.RecordFailedLogin(context.Background(), id, 3, time.Hour))
	}
	u, err := users.GetByID(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, u.LockedUntil, "setup: the account should be locked")
	require.Equal(t, 3, u.FailedLoginAttempts)
}

func TestUserRepo_ClearLockout_ClearsOnlyThatUser(t *testing.T) {
	users, _ := setupSessions(t)
	ctx := context.Background()
	a, err := users.CreateWithPassword(ctx, "a@example.com", "A", "hash")
	require.NoError(t, err)
	b, err := users.CreateWithPassword(ctx, "b@example.com", "B", "hash")
	require.NoError(t, err)
	lockOut(t, users, a.ID)
	lockOut(t, users, b.ID)

	require.NoError(t, users.ClearLockout(ctx, users.db.Pool, a.ID))

	gotA, err := users.GetByID(ctx, a.ID)
	require.NoError(t, err)
	assert.Zero(t, gotA.FailedLoginAttempts)
	assert.Nil(t, gotA.LockedUntil)
	gotB, err := users.GetByID(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, gotB.FailedLoginAttempts, "another user's lockout was cleared")
	assert.NotNil(t, gotB.LockedUntil, "another user's lockout was cleared")
}

func TestUserRepo_ClearLockout_MissingUser(t *testing.T) {
	users, _ := setupSessions(t)

	err := users.ClearLockout(context.Background(), users.db.Pool, 99999)

	require.ErrorIs(t, err, ErrNotFound)
}

// It runs on the caller's transaction, so a rolled-back reset leaves the lockout in place.
func TestUserRepo_ClearLockout_RollsBackWithItsTransaction(t *testing.T) {
	users, _ := setupSessions(t)
	ctx := context.Background()
	a, err := users.CreateWithPassword(ctx, "a@example.com", "A", "hash")
	require.NoError(t, err)
	lockOut(t, users, a.ID)

	tx, err := users.db.Pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, users.ClearLockout(ctx, tx, a.ID))
	require.NoError(t, tx.Rollback(ctx))

	got, err := users.GetByID(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, got.FailedLoginAttempts)
	assert.NotNil(t, got.LockedUntil)
}

// ResetLoginAttempts (login, change-password) keeps working through the same query.
func TestUserRepo_ResetLoginAttempts_StillClears(t *testing.T) {
	users, _ := setupSessions(t)
	ctx := context.Background()
	a, err := users.CreateWithPassword(ctx, "a@example.com", "A", "hash")
	require.NoError(t, err)
	lockOut(t, users, a.ID)

	require.NoError(t, users.ResetLoginAttempts(ctx, a.ID))

	got, err := users.GetByID(ctx, a.ID)
	require.NoError(t, err)
	assert.Zero(t, got.FailedLoginAttempts)
	assert.Nil(t, got.LockedUntil)
}

// An admin reset has no session to keep: an empty keep-token must end every session
// of that user (tokens are never empty) and no one else's.
func TestUserRepo_UpdatePasswordKeepSession_EmptyTokenEndsEverySession(t *testing.T) {
	users, tdb := setupSessions(t)
	ctx := context.Background()
	a, err := users.CreateWithPassword(ctx, "a@example.com", "A", "oldhash")
	require.NoError(t, err)
	b, err := users.CreateWithPassword(ctx, "b@example.com", "B", "hash")
	require.NoError(t, err)
	addSession(t, tdb, "a1", a.ID)
	addSession(t, tdb, "a2", a.ID)
	addSession(t, tdb, "a3", a.ID)
	addSession(t, tdb, "b1", b.ID)

	ended, err := users.UpdatePasswordKeepSession(ctx, users.db.Pool, a.ID, "newhash", "")

	require.NoError(t, err)
	assert.EqualValues(t, 3, ended)
	assert.Zero(t, liveSessions(t, tdb, a.ID))
	assert.Equal(t, 1, liveSessions(t, tdb, b.ID), "another user's session was revoked")
	got, err := users.GetByID(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, "newhash", got.PasswordHash)
}
