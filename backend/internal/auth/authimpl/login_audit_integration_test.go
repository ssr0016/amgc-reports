//go:build integration

package authimpl

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/token"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

func countAudit(t *testing.T, tdb *testutil.TestDB, action string) int {
	t.Helper()
	var n int
	require.NoError(t, tdb.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs WHERE action = $1`, action).Scan(&n))
	return n
}

// Runs against the migrated schema, so it also proves the CHECK allows the new actions.
func TestSessionAudit_LoginLogoutRefresh(t *testing.T) {
	svc, tdb, userID := newRefreshEnv(t)
	ctx := context.Background()

	p, err := svc.StartSession(ctx, userID, "10.0.0.9", "ua")
	require.NoError(t, err)
	assert.Equal(t, 1, countAudit(t, tdb, audit.ActionLogin))

	_, err = svc.Refresh(ctx, p.Refresh, "10.0.0.9", "ua")
	require.NoError(t, err)
	assert.Equal(t, 0, countAudit(t, tdb, audit.ActionLogout), "a refresh writes nothing")
	assert.Equal(t, 1, countAudit(t, tdb, audit.ActionLogin), "a refresh writes nothing")

	p2, err := svc.StartSession(ctx, userID, "10.0.0.9", "ua")
	require.NoError(t, err)
	require.NoError(t, svc.EndSession(ctx, p2.Refresh, "10.0.0.9"))
	require.NoError(t, svc.EndSession(ctx, p2.Refresh, "10.0.0.9"))
	assert.Equal(t, 1, countAudit(t, tdb, audit.ActionLogout), "only the logout that ended a live session")
}

type failingAudit struct{}

func (failingAudit) Insert(context.Context, audit.Log) error { return errors.New("check violation") }

// D-10: a login whose audit row cannot be written leaves no refresh token behind.
func TestStartSession_AuditFailureLeavesNoSession(t *testing.T) {
	_, tdb, userID := newRefreshEnv(t)
	ctx := context.Background()
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	svc := newService(userimpl.NewStore(db), nil, nil, failingAudit{}, db, slog.New(slog.DiscardHandler)).withTokens(
		token.New(token.Config{Secret: []byte("0123456789abcdef0123456789abcdef"), Issuer: "i", Audience: "a", AccessTTL: time.Minute}),
		NewRefreshStore(db), TokenSettings{RefreshTTL: time.Hour, RefreshIdleTTL: time.Hour})

	_, err := svc.StartSession(ctx, userID, "ip", "ua")
	require.Error(t, err)

	var n int
	require.NoError(t, tdb.Pool.QueryRow(ctx, `SELECT count(*) FROM refresh_tokens`).Scan(&n))
	assert.Zero(t, n)
}

func TestLogin_FailedLoginRowForUnknownEmail(t *testing.T) {
	svc, tdb, _ := newRefreshEnv(t)
	ctx := context.Background()

	_, err := svc.Login(ctx, "nobody@example.com", "whatever", "10.0.0.9")
	require.Error(t, err)

	var targetID, targetName *string
	var reason string
	require.NoError(t, tdb.Pool.QueryRow(ctx,
		`SELECT target_id, target_name, details->>'reason' FROM audit_logs WHERE action = 'LOGIN_FAILED'`).
		Scan(&targetID, &targetName, &reason))
	assert.Nil(t, targetID)
	assert.Nil(t, targetName)
	assert.Equal(t, audit.ReasonUnknownAccount, reason)
}
