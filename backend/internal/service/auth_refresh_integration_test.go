//go:build integration

package service

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/auth/token"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

func newRefreshEnv(t *testing.T) (*AuthService, *testutil.TestDB, int64) {
	t.Helper()
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := repository.NewUserRepo(db)
	u, err := users.CreateWithPassword(context.Background(), "ana@example.com", "Ana", "hash")
	require.NoError(t, err)
	svc := NewAuthService(users, nil, nil, nil, db, slog.New(slog.DiscardHandler)).
		WithTokens(token.New(token.Config{Secret: []byte("0123456789abcdef0123456789abcdef"), Issuer: "i", Audience: "a", AccessTTL: time.Minute}),
			repository.NewRefreshTokenRepo(db), TokenSettings{RefreshTTL: time.Hour, RefreshIdleTTL: time.Hour})
	return svc, tdb, u.ID
}

func liveTokens(t *testing.T, tdb *testutil.TestDB, family string) int {
	t.Helper()
	var n int
	require.NoError(t, tdb.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM refresh_tokens WHERE family_id = $1 AND revoked_at IS NULL`, family).Scan(&n))
	return n
}

// Two requests with the same refresh token at once: the row lock makes them run one
// after the other, so exactly one wins and the family keeps exactly one live token.
func TestRefresh_ConcurrentUseOfOneTokenHasOneWinner(t *testing.T) {
	svc, tdb, userID := newRefreshEnv(t)
	ctx := context.Background()
	p, err := svc.StartSession(ctx, userID, "ip", "ua")
	require.NoError(t, err)

	const n = 8
	var wg sync.WaitGroup
	results := make([]error, n)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, results[i] = svc.Refresh(ctx, p.Refresh, "ip", "ua")
		}()
	}
	wg.Wait()

	wins := 0
	for _, err := range results {
		if err == nil {
			wins++
		} else {
			assert.ErrorIs(t, err, ErrRefreshReplayed, "losers are replays of a just-rotated token")
		}
	}
	assert.Equal(t, 1, wins, "exactly one refresh of a token may succeed")
	assert.Equal(t, 1, liveTokens(t, tdb, p.SessionID), "the family must keep one live token")
}

// Reuse after the grace window revokes the family, including the legitimate newest token.
func TestRefresh_LateReuseRevokesTheFamily(t *testing.T) {
	svc, tdb, userID := newRefreshEnv(t)
	ctx := context.Background()
	first, err := svc.StartSession(ctx, userID, "ip", "ua")
	require.NoError(t, err)
	_, err = svc.Refresh(ctx, first.Refresh, "ip", "ua")
	require.NoError(t, err)
	_, err = tdb.Pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = NOW() - interval '1 minute' WHERE replaced_by IS NOT NULL`)
	require.NoError(t, err)

	_, err = svc.Refresh(ctx, first.Refresh, "ip", "ua")

	require.ErrorIs(t, err, ErrInvalidRefresh)
	assert.Zero(t, liveTokens(t, tdb, first.SessionID), "the revocation was rolled back with the error")
}

// pausingRefresh holds a rotation open at a chosen point: after it has inserted
// the new token and before it marks the old one replaced, so its transaction has
// uncommitted work and holds its locks.
type pausingRefresh struct {
	repository.RefreshTokenRepository
	reached chan struct{}
	release chan struct{}
}

func (p *pausingRefresh) MarkReplaced(ctx context.Context, id, replacedBy int64, at time.Time) error {
	close(p.reached)
	<-p.release
	return p.RefreshTokenRepository.MarkReplaced(ctx, id, replacedBy, at)
}

// A revocation that starts while a refresh is in flight must wait for it and then
// end the token the refresh created. Before the user-row lock, the revocation
// waited on the old token, skipped it once it was revoked, never saw the new row
// (not yet committed when its statement began), and left a live sign-in behind.
func TestRefresh_RevocationDuringRotationEndsTheNewToken(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	ctx := context.Background()
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := repository.NewUserRepo(db)
	u, err := users.CreateWithPassword(ctx, "ana@example.com", "Ana", "hash")
	require.NoError(t, err)

	pause := &pausingRefresh{
		RefreshTokenRepository: repository.NewRefreshTokenRepo(db),
		reached:                make(chan struct{}),
		release:                make(chan struct{}),
	}
	mgr := token.New(token.Config{Secret: []byte("0123456789abcdef0123456789abcdef"), Issuer: "i", Audience: "a", AccessTTL: time.Minute})
	set := TokenSettings{RefreshTTL: time.Hour, RefreshIdleTTL: time.Hour}
	log := slog.New(slog.DiscardHandler)
	start := NewAuthService(users, nil, nil, nil, db, log).WithTokens(mgr, repository.NewRefreshTokenRepo(db), set)
	svc := NewAuthService(users, nil, nil, nil, db, log).WithTokens(mgr, pause, set)

	p, err := start.StartSession(ctx, u.ID, "ip", "ua")
	require.NoError(t, err)

	refreshed := make(chan error, 1)
	go func() {
		_, err := svc.Refresh(ctx, p.Refresh, "ip", "ua")
		refreshed <- err
	}()
	<-pause.reached // the rotation holds its locks and has inserted the new token, uncommitted

	revoked := make(chan error, 1)
	go func() {
		_, err := users.RevokeSessions(ctx, u.ID)
		revoked <- err
	}()

	select {
	case err := <-revoked:
		t.Fatalf("the revocation did not wait for the rotation in flight (err=%v)", err)
	case <-time.After(500 * time.Millisecond): // still blocked on the user row: as intended
	}

	close(pause.release)
	require.NoError(t, <-refreshed, "the refresh was already past its checks")
	require.NoError(t, <-revoked)

	assert.Zero(t, liveTokens(t, tdb, p.SessionID), "a sign-in survived the revocation")
	var live int
	require.NoError(t, tdb.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL`, u.ID).Scan(&live))
	assert.Zero(t, live)
	got, err := users.GetWithRole(ctx, u.ID)
	require.NoError(t, err)
	assert.True(t, got.AccessRevoked(time.Now().Add(-time.Minute)), "tokens_valid_after was not moved")
}

// The other direction: a revocation that is already running makes a refresh wait,
// and the refresh then finds its token revoked.
func TestRefresh_WaitsForARevocationInFlight(t *testing.T) {
	svc, tdb, userID := newRefreshEnv(t)
	ctx := context.Background()
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := repository.NewUserRepo(db)
	p, err := svc.StartSession(ctx, userID, "ip", "ua")
	require.NoError(t, err)

	inRevocation, releaseRevocation := make(chan struct{}), make(chan struct{})
	revoked := make(chan error, 1)
	go func() {
		revoked <- db.InTransaction(ctx, func(ctx context.Context) error {
			if _, err := users.RevokeSessions(ctx, userID); err != nil {
				return err
			}
			close(inRevocation)
			<-releaseRevocation // hold the transaction open, row locks and all
			return nil
		})
	}()
	<-inRevocation

	refreshed := make(chan error, 1)
	go func() {
		_, err := svc.Refresh(ctx, p.Refresh, "ip", "ua")
		refreshed <- err
	}()
	select {
	case err := <-refreshed:
		t.Fatalf("the refresh did not wait for the revocation (err=%v)", err)
	case <-time.After(500 * time.Millisecond):
	}

	close(releaseRevocation)
	require.NoError(t, <-revoked)
	require.ErrorIs(t, <-refreshed, ErrInvalidRefresh)
	assert.Zero(t, liveTokens(t, tdb, p.SessionID))
}
