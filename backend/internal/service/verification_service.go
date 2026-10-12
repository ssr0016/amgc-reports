package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
)

type VerificationService struct {
	userRepo         repository.UserRepository
	verificationRepo repository.VerificationRepository
	log              *slog.Logger
}

func NewVerificationService(
	userRepo repository.UserRepository,
	verificationRepo repository.VerificationRepository,
	log *slog.Logger,
) *VerificationService {
	return &VerificationService{
		userRepo:         userRepo,
		verificationRepo: verificationRepo,
		log:              log,
	}
}

func (s *VerificationService) SendVerification(ctx context.Context, userID int64) (string, error) {
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

func (s *VerificationService) Verify(ctx context.Context, tokenStr string) error {
	token, err := s.verificationRepo.GetByToken(ctx, tokenStr)
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}
	if token == nil {
		return ErrInvalidToken
	}

	if !token.IsValid() {
		return ErrInvalidToken
	}

	// Claim the token before verifying the email, so concurrent requests
	// holding the same token cannot all pass. If the update below then fails
	// the token is spent and the user must request a new one.
	if err := s.verificationRepo.MarkUsed(ctx, token.ID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrInvalidToken
		}
		return fmt.Errorf("mark used: %w", err)
	}

	if err := s.userRepo.MarkEmailVerified(ctx, token.UserID); err != nil {
		return fmt.Errorf("mark verified: %w", err)
	}

	s.log.Info("email verified", "user_id", token.UserID)
	return nil
}
