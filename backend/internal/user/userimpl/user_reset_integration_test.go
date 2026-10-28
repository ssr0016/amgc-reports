//go:build integration

package userimpl

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/authimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

const oldResetPassword = "0ld-Passw0rd-Zq72-kB"

// resetTarget makes a user who has an old password, a lockout and some sessions.
func (e userSvcEnv) resetTarget(t *testing.T) *user.User {
	t.Helper()
	ctx := context.Background()
	hash, err := bcrypt.GenerateFromPassword([]byte(oldResetPassword), bcrypt.MinCost)
	require.NoError(t, err)
	u, err := e.users.CreateWithPassword(ctx, "target@example.com", "Target", string(hash))
	require.NoError(t, err)
	lockOut(t, e.users, u.ID)
	e.addSessions(t, u.ID, "t1", "t2")
	return u
}

func TestUserService_ResetPassword_WritesPasswordSessionsLockoutAndAuditInOneTransaction(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	actor := env.newAdmin(t, "actor@example.com", env.adminRole(t))
	target := env.resetTarget(t)
	other, err := env.users.CreateWithPassword(ctx, "other@example.com", "Other", "h")
	require.NoError(t, err)
	env.addSessions(t, other.ID, "o1")
	svc := env.service(auditimpl.NewStore(&database.DB{Pool: env.tdb.Pool, Builder: database.NewStatementBuilder()}))

	require.NoError(t, svc.ResetPassword(ctx, actor.ID, "10.0.0.9", target.ID, resetPassword))

	stored, err := env.users.GetByID(ctx, target.ID)
	require.NoError(t, err)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(resetPassword)), "the new password must verify")
	assert.Error(t, bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(oldResetPassword)), "the old password still works")
	assert.False(t, isLockedOut(t, env.users, target.ID), "the lockout survived the reset")
	assert.Zero(t, env.liveSessions(t, target.ID), "the target's sessions survived")
	assert.Equal(t, 1, env.liveSessions(t, other.ID), "another user's session was ended")

	var performedBy int64
	var targetName, details string
	require.NoError(t, env.tdb.Pool.QueryRow(ctx,
		`SELECT performed_by, target_name, details::text FROM audit_logs WHERE action = 'ADMIN_RESET_PASSWORD'`).
		Scan(&performedBy, &targetName, &details), "the CHECK must accept ADMIN_RESET_PASSWORD")
	assert.Equal(t, actor.ID, performedBy)
	assert.Equal(t, "target@example.com", targetName)
	assert.JSONEq(t, `{"sessions_ended":2}`, details)
	assert.NotContains(t, details, resetPassword)
}

func TestUserService_ResetPassword_AuditFailureRollsBackEverything(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	actor := env.newAdmin(t, "actor@example.com", env.adminRole(t))
	target := env.resetTarget(t)
	before, err := env.users.GetByID(ctx, target.ID)
	require.NoError(t, err)

	err = env.service(failingAudit{}).ResetPassword(ctx, actor.ID, "", target.ID, resetPassword)

	require.Error(t, err)
	after, err := env.users.GetByID(ctx, target.ID)
	require.NoError(t, err)
	assert.Equal(t, before.PasswordHash, after.PasswordHash, "the password changed although the audit failed")
	assert.Equal(t, 2, env.liveSessions(t, target.ID), "the sessions were ended without the reset")
	assert.True(t, isLockedOut(t, env.users, target.ID), "the lockout was cleared without the reset")
}

// What the admin is for: a locked-out worker signs in with the new password, and the old one is dead.
func TestUserService_ResetPassword_LockedOutUserSignsInWithTheNewPasswordOnly(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	actor := env.newAdmin(t, "actor@example.com", env.adminRole(t))
	target := env.resetTarget(t)
	authSvc := authimpl.NewService(env.users, nil, nil, auditimpl.NewMockStore(), nil, slog.New(slog.DiscardHandler), nil, nil, authimpl.TokenSettings{})

	_, err := authSvc.Login(ctx, target.Email, oldResetPassword, lockedOutIPs[0])
	require.ErrorIs(t, err, auth.ErrAccountLocked, "setup: locked before the reset")

	svc := env.service(auditimpl.NewStore(&database.DB{Pool: env.tdb.Pool, Builder: database.NewStatementBuilder()}))
	require.NoError(t, svc.ResetPassword(ctx, actor.ID, "", target.ID, resetPassword))

	_, err = authSvc.Login(ctx, target.Email, oldResetPassword, lockedOutIPs[0])
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials, "the old password still signs in")
	got, err := authSvc.Login(ctx, target.Email, resetPassword, lockedOutIPs[0])
	require.NoError(t, err, "the new password must sign in once the lockout is cleared")
	assert.Equal(t, target.ID, got.ID)
}
