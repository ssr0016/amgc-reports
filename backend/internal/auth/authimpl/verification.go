package authimpl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

type verificationService struct {
	userRepo         user.Store
	verificationRepo auth.VerificationStore
	log              *slog.Logger
}

// NewVerificationService returns the auth.VerificationService over its stores.
func NewVerificationService(
	userRepo user.Store,
	verificationRepo auth.VerificationStore,
	log *slog.Logger,
) auth.VerificationService {
	return &verificationService{
		userRepo:         userRepo,
		verificationRepo: verificationRepo,
		log:              log,
	}
}

func (s *verificationService) SendVerification(ctx context.Context, userID int64) (string, error) {
	token, err := s.verificationRepo.Create(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("create token: %w", err)
	}

	s.log.Info("verification token generated",
		"user_id", userID,
		"expires_at", token.ExpiresAt,
	)

	return token.Token, nil
}

func (s *verificationService) Verify(ctx context.Context, tokenStr string) error {
	token, err := s.verificationRepo.GetByToken(ctx, tokenStr)
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}
	if token == nil {
		return auth.ErrInvalidToken
	}

	if !token.IsValid() {
		return auth.ErrInvalidToken
	}

	// Claim the token before verifying the email, so concurrent requests
	// holding the same token cannot all pass. If the update below then fails
	// the token is spent and the user must request a new one.
	if err := s.verificationRepo.MarkUsed(ctx, token.ID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return auth.ErrInvalidToken
		}
		return fmt.Errorf("mark used: %w", err)
	}

	if err := s.userRepo.MarkEmailVerified(ctx, token.UserID); err != nil {
		return fmt.Errorf("mark verified: %w", err)
	}

	s.log.Info("email verified", "user_id", token.UserID)
	return nil
}
