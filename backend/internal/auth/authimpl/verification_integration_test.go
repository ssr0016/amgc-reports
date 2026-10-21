//go:build integration

package authimpl

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

// slowUserRepo delays MarkEmailVerified so that, without an atomic claim,
// every racer passes the token check before any of them marks it used.
type slowUserRepo struct{ user.Store }

func (r slowUserRepo) MarkEmailVerified(ctx context.Context, userID int64) error {
	time.Sleep(100 * time.Millisecond)
	return r.Store.MarkEmailVerified(ctx, userID)
}

// Several requests holding the same verification token race; the token is
// single use, so exactly one may succeed and the rest must read as invalid.
func TestVerificationService_Verify_ConcurrentUseOfOneToken(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := userimpl.NewStore(db)
	tokens := NewVerificationStore(db)
	svc := NewVerificationService(slowUserRepo{users}, tokens, discardLog)
	ctx := context.Background()

	user, err := users.CreateWithPassword(ctx, "race-verify@example.com", "Racer", "hash")
	require.NoError(t, err)
	token, err := tokens.Create(ctx, user.ID)
	require.NoError(t, err)

	const attempts = 8
	results := make([]error, attempts)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i] = svc.Verify(ctx, token.Token)
		}()
	}
	close(start)
	wg.Wait()

	succeeded := 0
	for _, err := range results {
		if err == nil {
			succeeded++
			continue
		}
		require.True(t, errors.Is(err, auth.ErrInvalidToken), "loser must see an invalid token, got: %v", err)
	}
	require.Equal(t, 1, succeeded, "exactly one concurrent verification may use the token")
}
