//go:build integration

package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/session"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

type storeFixture struct {
	tdb   *testutil.TestDB
	store *session.Store
	codec scs.Codec
}

func newStoreFixture(t *testing.T) *storeFixture {
	t.Helper()
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)

	codec := scs.GobCodec{}
	store := session.NewStore(tdb.Pool, codec)
	t.Cleanup(store.StopCleanup)
	return &storeFixture{tdb: tdb, store: store, codec: codec}
}

func (f *storeFixture) newUser(t *testing.T, email string) int64 {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("pw"), bcrypt.MinCost)
	require.NoError(t, err)
	users := repository.NewUserRepo(&database.DB{Pool: f.tdb.Pool, Builder: database.NewStatementBuilder()})
	user, err := users.CreateWithPassword(context.Background(), email, "Test", string(hash))
	require.NoError(t, err)
	return user.ID
}

// commit stores a session the way scs does: encoded data plus an expiry.
func (f *storeFixture) commit(t *testing.T, token string, values map[string]any) error {
	t.Helper()
	expiry := time.Now().Add(time.Hour)
	b, err := f.codec.Encode(expiry, values)
	require.NoError(t, err)
	return f.store.CommitCtx(context.Background(), token, b, expiry)
}

// revoke runs the statement UserRepo.UpdatePassword runs for a user.
func (f *storeFixture) revoke(t *testing.T, userID int64) {
	t.Helper()
	_, err := f.tdb.Pool.Exec(context.Background(),
		`UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	require.NoError(t, err)
}

func (f *storeFixture) found(t *testing.T, token string) bool {
	t.Helper()
	_, found, err := f.store.FindCtx(context.Background(), token)
	require.NoError(t, err)
	return found
}

// Revocation is by user, so every committed session must carry the user it
// belongs to; a session that never logged in belongs to nobody.
func TestSessionStore_CommitRecordsUserID(t *testing.T) {
	f := newStoreFixture(t)
	userID := f.newUser(t, "owner@example.com")
	ctx := context.Background()

	require.NoError(t, f.commit(t, "tok-user", map[string]any{"user_id": userID}))
	require.NoError(t, f.commit(t, "tok-anon", map[string]any{"csrf": "x"}))

	var got *int64
	require.NoError(t, f.tdb.Pool.QueryRow(ctx, `SELECT user_id FROM sessions WHERE token = 'tok-user'`).Scan(&got))
	require.NotNil(t, got, "user_id was not recorded")
	assert.Equal(t, userID, *got)

	require.NoError(t, f.tdb.Pool.QueryRow(ctx, `SELECT user_id FROM sessions WHERE token = 'tok-anon'`).Scan(&got))
	assert.Nil(t, got, "an anonymous session must not have a user")
}

func TestSessionStore_RevokedSessionIsNotFound(t *testing.T) {
	f := newStoreFixture(t)
	userID := f.newUser(t, "owner@example.com")
	require.NoError(t, f.commit(t, "tok", map[string]any{"user_id": userID}))
	require.True(t, f.found(t, "tok"), "precondition: the live session is found")

	f.revoke(t, userID)

	assert.False(t, f.found(t, "tok"), "a revoked session must not be found")
}

// Every request re-commits its session (idle timeout), as an upsert. A request
// that loaded the session before the revocation commits it afterwards; that
// must not bring the session back.
func TestSessionStore_LateCommitDoesNotResurrectRevokedSession(t *testing.T) {
	f := newStoreFixture(t)
	userID := f.newUser(t, "owner@example.com")
	require.NoError(t, f.commit(t, "tok", map[string]any{"user_id": userID}))
	f.revoke(t, userID)

	require.NoError(t, f.commit(t, "tok", map[string]any{"user_id": userID}), "a late commit is not an error")

	assert.False(t, f.found(t, "tok"), "a late commit resurrected a revoked session")
	var revoked bool
	require.NoError(t, f.tdb.Pool.QueryRow(context.Background(),
		`SELECT revoked_at IS NOT NULL FROM sessions WHERE token = 'tok'`).Scan(&revoked))
	assert.True(t, revoked, "a late commit cleared revoked_at")
}

// A revoked row is finished: a late commit must not rewrite its data, expiry
// or owner. (Findability is already blocked by revoked_at; this pins that the
// upsert leaves the row alone.)
func TestSessionStore_LateCommitLeavesRevokedRowUntouched(t *testing.T) {
	f := newStoreFixture(t)
	userID := f.newUser(t, "owner@example.com")
	other := f.newUser(t, "other@example.com")
	ctx := context.Background()
	require.NoError(t, f.commit(t, "tok", map[string]any{"user_id": userID}))
	f.revoke(t, userID)

	var dataBefore []byte
	var expiryBefore time.Time
	require.NoError(t, f.tdb.Pool.QueryRow(ctx,
		`SELECT data, expiry FROM sessions WHERE token = 'tok'`).Scan(&dataBefore, &expiryBefore))

	require.NoError(t, f.commit(t, "tok", map[string]any{"user_id": other, "extra": "changed"}))

	var dataAfter []byte
	var expiryAfter time.Time
	var ownerAfter int64
	require.NoError(t, f.tdb.Pool.QueryRow(ctx,
		`SELECT data, expiry, user_id FROM sessions WHERE token = 'tok'`).Scan(&dataAfter, &expiryAfter, &ownerAfter))
	assert.Equal(t, dataBefore, dataAfter, "a late commit rewrote a revoked session's data")
	assert.True(t, expiryBefore.Equal(expiryAfter), "a late commit moved a revoked session's expiry")
	assert.Equal(t, userID, ownerAfter, "a late commit changed a revoked session's owner")
}
