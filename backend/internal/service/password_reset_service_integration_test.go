//go:build integration

package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

// Several requests holding the same reset token race; the token is single
// use, so exactly one may succeed and the rest must read as an invalid token.
func TestPasswordResetService_ResetPassword_ConcurrentUseOfOneToken(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := repository.NewUserRepo(db)
	resets := repository.NewPasswordResetRepo(db)
	svc := NewPasswordResetService(users, resets, discardLog)
	ctx := context.Background()

	user, err := users.CreateWithPassword(ctx, "race@example.com", "Racer", "original-hash")
	require.NoError(t, err)
	token, err := resets.Create(ctx, user.ID)
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
			results[i] = svc.ResetPassword(ctx, token.Token, testPassword)
		}()
	}
	close(start)
	wg.Wait()

	succeeded := 0
	for _, err := range results {
		switch {
		case err == nil:
			succeeded++
		default:
			require.True(t, errors.Is(err, ErrInvalidResetToken), "loser must see an invalid token, got: %v", err)
		}
	}
	require.Equal(t, 1, succeeded, "exactly one concurrent reset may use the token")
}
