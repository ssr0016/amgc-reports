package authimpl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

// stubPasswordResetRepo hands out one valid token whose MarkUsed result is
// chosen by the test.
type stubPasswordResetRepo struct {
	markUsedErr error
}

func (r *stubPasswordResetRepo) Create(context.Context, int64) (*auth.PasswordReset, error) {
	return nil, errors.New("not used")
}

func (r *stubPasswordResetRepo) GetByToken(context.Context, string) (*auth.PasswordReset, error) {
	return &auth.PasswordReset{ID: 1, UserID: 42, Token: "t", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (r *stubPasswordResetRepo) MarkUsed(context.Context, int64) error { return r.markUsedErr }

func (r *stubPasswordResetRepo) DeleteExpired(context.Context) (int64, error) { return 0, nil }

// The token is claimed before the password changes: a request that loses the
// claim must not touch the password.
func TestPasswordResetService_ResetPassword_LostClaimDoesNotChangePassword(t *testing.T) {
	users := userimpl.NewMockStore()
	updated := false
	users.UpdatePasswordFunc = func(context.Context, int64, string) error {
		updated = true
		return nil
	}
	resets := &stubPasswordResetRepo{markUsedErr: database.ErrNotFound}
	svc := NewPasswordResetService(users, resets, discardLog)

	err := svc.ResetPassword(context.Background(), "t", testPassword)

	if !errors.Is(err, auth.ErrInvalidResetToken) {
		t.Errorf("ResetPassword() error = %v, want auth.ErrInvalidResetToken", err)
	}
	if updated {
		t.Error("password was updated although the token claim was lost")
	}
}

func TestPasswordResetService_ResetPassword_ClaimFailureStaysAnError(t *testing.T) {
	users := userimpl.NewMockStore()
	resets := &stubPasswordResetRepo{markUsedErr: errors.New("connection refused")}
	svc := NewPasswordResetService(users, resets, discardLog)

	err := svc.ResetPassword(context.Background(), "t", testPassword)

	if err == nil || errors.Is(err, auth.ErrInvalidResetToken) {
		t.Errorf("ResetPassword() error = %v, want a non-token error", err)
	}
}

// createCountingResetRepo counts the tokens it is asked to create.
type createCountingResetRepo struct {
	stubPasswordResetRepo
	created int
}

func (r *createCountingResetRepo) Create(context.Context, int64) (*auth.PasswordReset, error) {
	r.created++
	return &auth.PasswordReset{ExpiresAt: time.Now().Add(time.Hour)}, nil
}

// A deleted user is handled like an unknown account (D-A11): no token, no error.
func TestPasswordResetService_RequestReset_DeletedUserGetsNoToken(t *testing.T) {
	users := userimpl.NewMockStore()
	u, _ := users.CreateWithPassword(context.Background(), "d@example.com", "D", "hash")
	now := time.Now()
	u.DeletedAt = &now
	resets := &createCountingResetRepo{}
	svc := NewPasswordResetService(users, resets, discardLog)

	if err := svc.RequestReset(context.Background(), "d@example.com"); err != nil {
		t.Fatalf("RequestReset() error = %v, want nil", err)
	}
	if resets.created != 0 {
		t.Errorf("created %d reset tokens for a deleted user, want 0", resets.created)
	}
}
