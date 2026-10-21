package authimpl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

type passwordResetService struct {
	userRepo          user.Store
	passwordResetRepo auth.PasswordResetStore
	log               *slog.Logger
}

// NewPasswordResetService returns the auth.PasswordResetService over its stores.
func NewPasswordResetService(
	userRepo user.Store,
	passwordResetRepo auth.PasswordResetStore,
	log *slog.Logger,
) auth.PasswordResetService {
	return &passwordResetService{
		userRepo:          userRepo,
		passwordResetRepo: passwordResetRepo,
		log:               log,
	}
}

func (s *passwordResetService) RequestReset(ctx context.Context, email string) error {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		s.log.Info("password reset requested for unknown account")
		return nil
	}

	token, err := s.passwordResetRepo.Create(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("create reset token: %w", err)
	}

	s.log.Info("password reset token generated",
		"user_id", user.ID,
		"expires_at", token.ExpiresAt,
	)

	return nil
}

func (s *passwordResetService) ResetPassword(ctx context.Context, tokenStr, newPassword string) error {
	// Checked first: it needs no lookup, and bcrypt would otherwise cut the password at 72 bytes.
	if len(newPassword) > auth.MaxPasswordBytes {
		return auth.ErrPasswordTooLong
	}
	token, err := s.passwordResetRepo.GetByToken(ctx, tokenStr)
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}
	if token == nil {
		return auth.ErrInvalidResetToken
	}

	if !token.IsValid() {
		return auth.ErrInvalidResetToken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	// Claim the token before changing the password, so concurrent requests
	// holding the same token cannot all pass. If the update below then fails
	// the token is spent and the user must request a new one; the alternative
	// order would leave a used-up token valid.
	if err := s.passwordResetRepo.MarkUsed(ctx, token.ID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return auth.ErrInvalidResetToken
		}
		return fmt.Errorf("mark used: %w", err)
	}

	if err := s.userRepo.UpdatePassword(ctx, token.UserID, string(hash)); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	s.log.Info("password reset successful", "user_id", token.UserID)
	return nil
}
