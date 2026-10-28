//go:build integration

package userimpl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lockedOutIPs are the IPs lockOut locks a user out from.
var lockedOutIPs = []string{"1.1.1.1", "2.2.2.2"}

func lockOut(t *testing.T, users user.Store, id int64) {
	t.Helper()
	for _, ip := range lockedOutIPs {
		require.NoError(t, users.RecordFailedLogin(context.Background(), id, ip, 1, time.Hour))
	}
	require.True(t, isLockedOut(t, users, id), "setup: the account should be locked")
}

// isLockedOut reports whether id is locked out from any of lockedOutIPs.
func isLockedOut(t *testing.T, users user.Store, id int64) bool {
	t.Helper()
	for _, ip := range lockedOutIPs {
		locked, err := users.LoginLocked(context.Background(), id, ip)
		require.NoError(t, err)
		if locked {
			return true
		}
	}
	return false
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

	require.NoError(t, users.ClearLockout(ctx, a.ID))

	assert.False(t, isLockedOut(t, users, a.ID), "a lock from some IP survived")
	assert.True(t, isLockedOut(t, users, b.ID), "another user's lockout was cleared")
}

// It joins the caller's transaction, so a rolled-back reset leaves the lockout in place.
func TestUserRepo_ClearLockout_RollsBackWithItsTransaction(t *testing.T) {
	users, _ := setupSessions(t)
	ctx := context.Background()
	a, err := users.CreateWithPassword(ctx, "a@example.com", "A", "hash")
	require.NoError(t, err)
	lockOut(t, users, a.ID)

	err = users.(*store).db.InTransaction(ctx, func(ctx context.Context) error {
		require.NoError(t, users.ClearLockout(ctx, a.ID))
		return errors.New("rollback")
	})
	require.Error(t, err)

	assert.True(t, isLockedOut(t, users, a.ID))
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

	ended, err := users.UpdatePasswordKeepSession(ctx, a.ID, "newhash", "")

	require.NoError(t, err)
	assert.EqualValues(t, 3, ended)
	assert.Zero(t, liveSessions(t, tdb, a.ID))
	assert.Equal(t, 1, liveSessions(t, tdb, b.ID), "another user's session was revoked")
	got, err := users.GetByID(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, "newhash", got.PasswordHash)
}
