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
