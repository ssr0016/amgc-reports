//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

// These tests need the columns the goose migrations add to sessions, which the
// hand-written RunMigrations schema does not have.
func setupSessions(t *testing.T) (*UserRepo, *testutil.TestDB) {
	t.Helper()
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	return NewUserRepo(&database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}), tdb
}

func addSession(t *testing.T, tdb *testutil.TestDB, token string, userID int64) {
	t.Helper()
	_, err := tdb.Pool.Exec(context.Background(),
		`INSERT INTO sessions (token, data, expiry, user_id) VALUES ($1, '\x00', NOW() + interval '1 hour', $2)`,
		token, userID)
	require.NoError(t, err)
}

func liveSessions(t *testing.T, tdb *testutil.TestDB, userID int64) int {
	t.Helper()
	var n int
	require.NoError(t, tdb.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM sessions WHERE user_id = $1 AND revoked_at IS NULL`, userID).Scan(&n))
	return n
}

// A password change ends every session of that user, and only theirs.
func TestUserRepo_UpdatePassword_RevokesUsersSessions(t *testing.T) {
	users, tdb := setupSessions(t)
	ctx := context.Background()
	a, err := users.CreateWithPassword(ctx, "a@example.com", "A", "hash")
	require.NoError(t, err)
	b, err := users.CreateWithPassword(ctx, "b@example.com", "B", "hash")
	require.NoError(t, err)
	addSession(t, tdb, "a1", a.ID)
	addSession(t, tdb, "a2", a.ID)
	addSession(t, tdb, "b1", b.ID)

	require.NoError(t, users.UpdatePassword(ctx, a.ID, "newhash"))

	assert.Zero(t, liveSessions(t, tdb, a.ID), "the user's sessions survived the password change")
	assert.Equal(t, 1, liveSessions(t, tdb, b.ID), "another user's session was revoked")
}

func TestUserRepo_UpdatePassword_MissingUserRevokesNothing(t *testing.T) {
	users, tdb := setupSessions(t)
	ctx := context.Background()
	b, err := users.CreateWithPassword(ctx, "b@example.com", "B", "hash")
	require.NoError(t, err)
	addSession(t, tdb, "b1", b.ID)

	err = users.UpdatePassword(ctx, 99999, "newhash")

	require.ErrorIs(t, err, ErrNotFound)
	assert.Equal(t, 1, liveSessions(t, tdb, b.ID), "a failed update revoked someone's session")
}

// Deleting a user must not leave sessions that point at nobody.
func TestSessions_DeletingUserRemovesTheirSessions(t *testing.T) {
	users, tdb := setupSessions(t)
	ctx := context.Background()
	a, err := users.CreateWithPassword(ctx, "a@example.com", "A", "hash")
	require.NoError(t, err)
	addSession(t, tdb, "a1", a.ID)

	_, err = tdb.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, a.ID)
	require.NoError(t, err)

	var n int
	require.NoError(t, tdb.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM sessions WHERE token = 'a1'`).Scan(&n))
	assert.Zero(t, n, "the session outlived its user")
}

// The password change and the revocation are one unit: if the sessions cannot
// be revoked, the password must not change either, or a reset would report
// success while the attacker's session lives on.
func TestUserRepo_UpdatePassword_RollbackOnRevokeFailure(t *testing.T) {
	users, tdb := setupSessions(t)
	ctx := context.Background()
	a, err := users.CreateWithPassword(ctx, "a@example.com", "A", "oldhash")
	require.NoError(t, err)
	addSession(t, tdb, "a1", a.ID)

	// Make every UPDATE of sessions fail, which is what the revoke runs.
	_, err = tdb.Pool.Exec(ctx, `
		CREATE FUNCTION block_session_update() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'session update blocked'; END $$;
		CREATE TRIGGER sessions_block_update BEFORE UPDATE ON sessions
			FOR EACH ROW EXECUTE FUNCTION block_session_update();`)
	require.NoError(t, err)

	err = users.UpdatePassword(ctx, a.ID, "newhash")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
	got, err := users.GetByEmail(ctx, "a@example.com")
	require.NoError(t, err)
	assert.Equal(t, "oldhash", got.PasswordHash, "the password changed although the revocation failed")
	assert.Equal(t, 1, liveSessions(t, tdb, a.ID))
}

// A signed-in user changing their own password ends their other sessions but
// keeps the one they are using, and reports how many it ended.
func TestUserRepo_UpdatePasswordKeepSession(t *testing.T) {
	users, tdb := setupSessions(t)
	ctx := context.Background()
	a, err := users.CreateWithPassword(ctx, "a@example.com", "A", "oldhash")
	require.NoError(t, err)
	b, err := users.CreateWithPassword(ctx, "b@example.com", "B", "hash")
	require.NoError(t, err)
	addSession(t, tdb, "a-here", a.ID)
	addSession(t, tdb, "a-phone", a.ID)
	addSession(t, tdb, "a-laptop", a.ID)
	addSession(t, tdb, "b1", b.ID)

	ended, err := users.UpdatePasswordKeepSession(ctx, tdb.Pool, a.ID, "newhash", "a-here")

	require.NoError(t, err)
	assert.EqualValues(t, 2, ended)
	got, err := users.GetByEmail(ctx, "a@example.com")
	require.NoError(t, err)
	assert.Equal(t, "newhash", got.PasswordHash)
	assert.Equal(t, 1, liveSessions(t, tdb, a.ID), "only the current session should be left")
	var kept string
	require.NoError(t, tdb.Pool.QueryRow(ctx,
		`SELECT token FROM sessions WHERE user_id = $1 AND revoked_at IS NULL`, a.ID).Scan(&kept))
	assert.Equal(t, "a-here", kept)
	assert.Equal(t, 1, liveSessions(t, tdb, b.ID), "another user's session was revoked")
}

func TestUserRepo_UpdatePasswordKeepSession_MissingUser(t *testing.T) {
	users, tdb := setupSessions(t)
	ctx := context.Background()
	b, err := users.CreateWithPassword(ctx, "b@example.com", "B", "hash")
	require.NoError(t, err)
	addSession(t, tdb, "b1", b.ID)

	_, err = users.UpdatePasswordKeepSession(ctx, tdb.Pool, 99999, "newhash", "x")

	require.ErrorIs(t, err, ErrNotFound)
	assert.Equal(t, 1, liveSessions(t, tdb, b.ID))
}

// Run inside the caller's transaction, a failure to revoke rolls the password back with it.
func TestUserRepo_UpdatePasswordKeepSession_RollsBackWithItsTransaction(t *testing.T) {
	users, tdb := setupSessions(t)
	ctx := context.Background()
	a, err := users.CreateWithPassword(ctx, "a@example.com", "A", "oldhash")
	require.NoError(t, err)
	addSession(t, tdb, "a-here", a.ID)
	addSession(t, tdb, "a-phone", a.ID)

	boom := errors.New("audit failed")
	err = WithTx(ctx, tdb.Pool, func(q Querier) error {
		if _, err := users.UpdatePasswordKeepSession(ctx, q, a.ID, "newhash", "a-here"); err != nil {
			return err
		}
		return boom // what a failed audit insert would do
	})

	require.ErrorIs(t, err, boom)
	got, err := users.GetByEmail(ctx, "a@example.com")
	require.NoError(t, err)
	assert.Equal(t, "oldhash", got.PasswordHash)
	assert.Equal(t, 2, liveSessions(t, tdb, a.ID), "the revocation survived the rollback")
}
