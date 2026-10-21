package authimpl

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/token"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

// directTx runs fn without a transaction, as the in-memory mocks have none.
type directTx struct{}

func (directTx) InTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type tokenFixture struct {
	svc     *service
	users   *userimpl.MockStore
	refresh *MockRefreshStore
	userID  int64
}

func newTokenFixture(t *testing.T, set TokenSettings) *tokenFixture {
	t.Helper()
	users := userimpl.NewMockStore()
	u, err := users.CreateWithPassword(context.Background(), "ana@example.com", "Ana", "hash")
	require.NoError(t, err)
	refresh := NewMockRefreshStore()
	svc := newService(users, nil, nil, nil, directTx{}, slog.New(slog.DiscardHandler)).withTokens(token.New(token.Config{Secret: []byte("0123456789abcdef0123456789abcdef"), Issuer: "i", Audience: "a", AccessTTL: time.Minute}),
		refresh, set)
	return &tokenFixture{svc: svc, users: users, refresh: refresh, userID: u.ID}
}

var defaultSet = TokenSettings{RefreshTTL: time.Hour, RefreshIdleTTL: 30 * time.Minute}

func TestStartSession_IssuesTokensForANewFamily(t *testing.T) {
	f := newTokenFixture(t, defaultSet)
	a, err := f.svc.StartSession(context.Background(), f.userID, "1.2.3.4", "ua")
	require.NoError(t, err)
	b, err := f.svc.StartSession(context.Background(), f.userID, "1.2.3.4", "ua")
	require.NoError(t, err)
	require.NotEqual(t, a.SessionID, b.SessionID, "each sign-in is its own family")
	require.NotEqual(t, a.Refresh, b.Refresh)
	require.NotEmpty(t, a.Access)
	require.WithinDuration(t, time.Now().Add(time.Hour), a.RefreshExpires, 5*time.Second)
}

func TestRefresh_RotatesAndKeepsTheFamilyAndItsAbsoluteExpiry(t *testing.T) {
	f := newTokenFixture(t, defaultSet)
	ctx := context.Background()
	first, err := f.svc.StartSession(ctx, f.userID, "ip", "ua")
	require.NoError(t, err)

	next, err := f.svc.Refresh(ctx, first.Refresh, "ip", "ua")
	require.NoError(t, err)
	require.NotEqual(t, first.Refresh, next.Refresh)
	require.Equal(t, first.SessionID, next.SessionID)
	require.True(t, first.RefreshExpires.Equal(next.RefreshExpires), "a refresh must not extend the absolute lifetime")

	_, err = f.svc.Refresh(ctx, first.Refresh, "ip", "ua")
	require.ErrorIs(t, err, auth.ErrRefreshReplayed, "the old token must be spent")
	require.NotErrorIs(t, err, auth.ErrInvalidRefresh, "a replay is not an invalid token: the caller must keep the cookies")
	_, err = f.svc.Refresh(ctx, next.Refresh, "ip", "ua")
	require.NoError(t, err, "a quick replay of the old token must not end the sign-in (two tabs)")
}

func TestRefresh_Refuses(t *testing.T) {
	ctx := context.Background()

	t.Run("unknown token", func(t *testing.T) {
		f := newTokenFixture(t, defaultSet)
		_, err := f.svc.Refresh(ctx, "made-up", "ip", "ua")
		require.ErrorIs(t, err, auth.ErrInvalidRefresh)
	})
	t.Run("past the absolute expiry", func(t *testing.T) {
		f := newTokenFixture(t, TokenSettings{RefreshTTL: -time.Second, RefreshIdleTTL: time.Hour})
		p, err := f.svc.StartSession(ctx, f.userID, "ip", "ua")
		require.NoError(t, err)
		_, err = f.svc.Refresh(ctx, p.Refresh, "ip", "ua")
		require.ErrorIs(t, err, auth.ErrInvalidRefresh)
	})
	t.Run("idle too long", func(t *testing.T) {
		f := newTokenFixture(t, TokenSettings{RefreshTTL: time.Hour, RefreshIdleTTL: -time.Second})
		p, err := f.svc.StartSession(ctx, f.userID, "ip", "ua")
		require.NoError(t, err)
		_, err = f.svc.Refresh(ctx, p.Refresh, "ip", "ua")
		require.ErrorIs(t, err, auth.ErrInvalidRefresh)
	})
	t.Run("account no longer active ends the sign-in", func(t *testing.T) {
		f := newTokenFixture(t, defaultSet)
		p, err := f.svc.StartSession(ctx, f.userID, "ip", "ua")
		require.NoError(t, err)
		f.users.GetByIDFunc = func(context.Context, int64) (*user.User, error) {
			return &user.User{ID: f.userID, Status: user.UserStatusInactive}, nil
		}
		_, err = f.svc.Refresh(ctx, p.Refresh, "ip", "ua")
		require.ErrorIs(t, err, auth.ErrUnauthorized)
		f.users.GetByIDFunc = nil
		_, err = f.svc.Refresh(ctx, p.Refresh, "ip", "ua")
		require.ErrorIs(t, err, auth.ErrInvalidRefresh, "the family was revoked, so even an active account cannot use it")
	})
	t.Run("a store failure is not an invalid token", func(t *testing.T) {
		f := newTokenFixture(t, defaultSet)
		f.users.GetByIDFunc = func(context.Context, int64) (*user.User, error) { return nil, errors.New("db down") }
		p, err := f.svc.StartSession(ctx, f.userID, "ip", "ua")
		require.NoError(t, err)
		_, err = f.svc.Refresh(ctx, p.Refresh, "ip", "ua")
		require.Error(t, err)
		require.NotErrorIs(t, err, auth.ErrInvalidRefresh)
	})
}

func TestEndSession(t *testing.T) {
	f := newTokenFixture(t, defaultSet)
	ctx := context.Background()
	p, err := f.svc.StartSession(ctx, f.userID, "ip", "ua")
	require.NoError(t, err)

	require.NoError(t, f.svc.EndSession(ctx, p.Refresh))
	_, err = f.svc.Refresh(ctx, p.Refresh, "ip", "ua")
	require.ErrorIs(t, err, auth.ErrInvalidRefresh)
	require.NoError(t, f.svc.EndSession(ctx, p.Refresh), "ending twice is fine")
	require.NoError(t, f.svc.EndSession(ctx, "made-up"), "an unknown token is fine")
}

func TestRefresh_ReplayVersusReuseVersusRevoked(t *testing.T) {
	ctx := context.Background()
	f := newTokenFixture(t, defaultSet)
	first, err := f.svc.StartSession(ctx, f.userID, "ip", "ua")
	require.NoError(t, err)
	_, err = f.svc.Refresh(ctx, first.Refresh, "ip", "ua")
	require.NoError(t, err)

	// Rotated a moment ago: a replay, family untouched.
	_, err = f.svc.Refresh(ctx, first.Refresh, "ip", "ua")
	require.ErrorIs(t, err, auth.ErrRefreshReplayed)

	// Rotated long ago: theft, family revoked, plainly invalid.
	f.refresh.AgeRevocations(time.Minute)
	_, err = f.svc.Refresh(ctx, first.Refresh, "ip", "ua")
	require.ErrorIs(t, err, auth.ErrInvalidRefresh)
	require.NotErrorIs(t, err, auth.ErrRefreshReplayed)

	// Revoked without a successor (logout): invalid, never a replay.
	g, err := f.svc.StartSession(ctx, f.userID, "ip", "ua")
	require.NoError(t, err)
	require.NoError(t, f.svc.EndSession(ctx, g.Refresh))
	_, err = f.svc.Refresh(ctx, g.Refresh, "ip", "ua")
	require.ErrorIs(t, err, auth.ErrInvalidRefresh)
}
