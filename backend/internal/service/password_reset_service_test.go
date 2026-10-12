package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

// stubPasswordResetRepo hands out one valid token whose MarkUsed result is
// chosen by the test.
type stubPasswordResetRepo struct {
	markUsedErr error
}

func (r *stubPasswordResetRepo) Create(context.Context, int64) (*model.PasswordReset, error) {
	return nil, errors.New("not used")
}

func (r *stubPasswordResetRepo) GetByToken(context.Context, string) (*model.PasswordReset, error) {
	return &model.PasswordReset{ID: 1, UserID: 42, Token: "t", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (r *stubPasswordResetRepo) MarkUsed(context.Context, int64) error { return r.markUsedErr }

func (r *stubPasswordResetRepo) DeleteExpired(context.Context) (int64, error) { return 0, nil }

// The token is claimed before the password changes: a request that loses the
// claim must not touch the password.
func TestPasswordResetService_ResetPassword_LostClaimDoesNotChangePassword(t *testing.T) {
	users := repository.NewMockUserRepo()
	updated := false
	users.UpdatePasswordFunc = func(context.Context, int64, string) error {
		updated = true
		return nil
	}
	resets := &stubPasswordResetRepo{markUsedErr: repository.ErrNotFound}
	svc := NewPasswordResetService(users, resets, discardLog)

	err := svc.ResetPassword(context.Background(), "t", testPassword)

	if !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("ResetPassword() error = %v, want ErrInvalidResetToken", err)
	}
	if updated {
		t.Error("password was updated although the token claim was lost")
	}
}

func TestPasswordResetService_ResetPassword_ClaimFailureStaysAnError(t *testing.T) {
	users := repository.NewMockUserRepo()
	resets := &stubPasswordResetRepo{markUsedErr: errors.New("connection refused")}
	svc := NewPasswordResetService(users, resets, discardLog)

	err := svc.ResetPassword(context.Background(), "t", testPassword)

	if err == nil || errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("ResetPassword() error = %v, want a non-token error", err)
	}
}
