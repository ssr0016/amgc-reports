package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

const newPassword = "N3wP4ssw0rd-Zq81-mX"

type changeFixture struct {
	svc   *AuthService
	users *repository.MockUserRepo
	audit *repository.MockAuditRepo
	user  *model.User
	kept  string
	ended int64
}

func newChangeFixture(t *testing.T) *changeFixture {
	t.Helper()
	f := &changeFixture{users: repository.NewMockUserRepo(), audit: repository.NewMockAuditRepo(), ended: 2}
	f.user = seedUser(t, f.users, "ana@example.com", "Ana")
	f.users.GetWithRoleFunc = func(_ context.Context, id int64) (*model.User, error) {
		u, err := f.users.GetByID(context.Background(), id)
		if u != nil {
			u.Role = &model.Role{Name: "user"}
		}
		return u, err
	}
	f.users.UpdatePasswordKeepSessionFunc = func(_ context.Context, _ repository.Querier, id int64, hash, keep string) (int64, error) {
		f.kept = keep
		return f.ended, f.users.UpdatePassword(context.Background(), id, hash)
	}
	f.svc = NewAuthService(f.users, nil, nil, f.audit, fakeTx{}, discardLog)
	return f
}

func (f *changeFixture) change(current, next string) error {
	return f.svc.ChangePassword(context.Background(), f.user.ID, "10.0.0.1", "tok-current", current, next)
}

func (f *changeFixture) stored(t *testing.T) *model.User {
	t.Helper()
	u, err := f.users.GetByID(context.Background(), f.user.ID)
	require.NoError(t, err)
	return u
}

func TestChangePassword_Success(t *testing.T) {
	f := newChangeFixture(t)
	require.NoError(t, f.change(testPassword, newPassword))

	u := f.stored(t)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(newPassword)), "the new password is stored, hashed")
	assert.Error(t, bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(testPassword)))
	assert.Equal(t, "tok-current", f.kept, "the caller's own session is the one kept")
}

// The same counter as login: a stolen session must not be able to guess the current password.
func TestChangePassword_WrongCurrentCountsTowardLockout(t *testing.T) {
	f := newChangeFixture(t)
	for i := 0; i < maxFailedLoginAttempts; i++ {
		require.ErrorIs(t, f.change("wrong-password", newPassword), ErrInvalidCredentials)
	}
	assert.Equal(t, maxFailedLoginAttempts, f.stored(t).FailedLoginAttempts)
	assert.NotNil(t, f.stored(t).LockedUntil, "the account locks like it does for logins")
	assert.Empty(t, f.audit.Entries(), "a failed attempt is not a password change")
}

func TestChangePassword_LockedAccountIsRefusedEvenWithTheRightPassword(t *testing.T) {
	f := newChangeFixture(t)
	for i := 0; i < maxFailedLoginAttempts; i++ {
		_ = f.change("wrong-password", newPassword)
	}
	require.ErrorIs(t, f.change(testPassword, newPassword), ErrAccountLocked)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(f.stored(t).PasswordHash), []byte(testPassword)), "nothing changed")
}

func TestChangePassword_SuccessResetsTheCounter(t *testing.T) {
	f := newChangeFixture(t)
	for i := 0; i < maxFailedLoginAttempts-1; i++ {
		_ = f.change("wrong-password", newPassword)
	}
	require.NoError(t, f.change(testPassword, newPassword))
	assert.Zero(t, f.stored(t).FailedLoginAttempts)
	assert.Nil(t, f.stored(t).LockedUntil)
}

// Checked only after the current password is verified, or the answer would tell
// whoever holds a session what the current password is.
func TestChangePassword_SamePasswordIsRefused(t *testing.T) {
	f := newChangeFixture(t)
	require.ErrorIs(t, f.change(testPassword, testPassword), ErrPasswordUnchanged)
	assert.Empty(t, f.audit.Entries())

	require.ErrorIs(t, f.change("wrong-password", "wrong-password"), ErrInvalidCredentials,
		"with a wrong current password the answer must not say whether the new one matches")
}

// bcrypt reads 72 bytes and no more; the count is of bytes, not characters.
func TestChangePassword_Maximum72Bytes(t *testing.T) {
	f := newChangeFixture(t)
	ok := strings.Repeat("aB3$", 18) // exactly 72 bytes
	require.NoError(t, f.change(testPassword, ok))
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(f.stored(t).PasswordHash), []byte(ok)))

	f = newChangeFixture(t)
	require.ErrorIs(t, f.change(testPassword, ok+"x"), ErrPasswordTooLong)
	require.ErrorIs(t, f.change(testPassword, strings.Repeat("é", 37)), ErrPasswordTooLong, "37 two-byte characters is 74 bytes")
}

func TestChangePassword_AuditRowHasNoSecrets(t *testing.T) {
	f := newChangeFixture(t)
	require.NoError(t, f.change(testPassword, newPassword))

	entries := f.audit.Entries()
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, model.AuditChangePassword, e.Action)
	assert.Equal(t, model.AuditTargetTypeUser, e.TargetType)
	assert.Equal(t, "ana@example.com", *e.TargetName)
	assert.Equal(t, f.user.ID, *e.PerformedBy)
	assert.Equal(t, "user", *e.PerformedByRole)
	assert.Equal(t, "10.0.0.1", *e.IPAddress)
	assert.Equal(t, map[string]any{"sessions_ended": int64(2)}, e.Details)
	for _, secret := range []string{testPassword, newPassword, f.stored(t).PasswordHash, "tok-current"} {
		assert.NotContains(t, e.Details, secret)
		assert.NotContains(t, *e.TargetName, secret)
	}
}

// The password and the audit row are one unit: no row, no change.
func TestChangePassword_FailedAuditRollsBack(t *testing.T) {
	f := newChangeFixture(t)
	boom := errors.New("audit down")
	f.audit.InsertFunc = func(context.Context, repository.Querier, model.AuditLog) error { return boom }

	var committed bool
	f.users.UpdatePasswordKeepSessionFunc = func(_ context.Context, _ repository.Querier, id int64, hash, _ string) (int64, error) {
		committed = true
		return 0, nil
	}
	var rolledBack bool
	f.svc = NewAuthService(f.users, nil, nil, f.audit, rollbackTx{&rolledBack}, discardLog)

	require.ErrorIs(t, f.change(testPassword, newPassword), boom)
	assert.True(t, committed, "the update ran inside the transaction")
	assert.True(t, rolledBack, "and the transaction was rolled back, not committed")
}

func TestChangePassword_UnknownUserIsUnauthorized(t *testing.T) {
	f := newChangeFixture(t)
	f.users.GetWithRoleFunc = func(context.Context, int64) (*model.User, error) { return nil, nil }
	require.ErrorIs(t, f.change(testPassword, newPassword), ErrUnauthorized)
}

// rollbackTx reports whether the work failed, as a real transaction would roll back.
type rollbackTx struct{ rolledBack *bool }

func (r rollbackTx) WithTx(_ context.Context, fn func(repository.Querier) error) error {
	if err := fn(nil); err != nil {
		*r.rolledBack = true
		return err
	}
	return nil
}
