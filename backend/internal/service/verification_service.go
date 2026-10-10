package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ssr0016/template/internal/repository"
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
		"token", token.Token,
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

	if err := s.userRepo.MarkEmailVerified(ctx, token.UserID); err != nil {
		return fmt.Errorf("mark verified: %w", err)
	}

	if err := s.verificationRepo.MarkUsed(ctx, token.ID); err != nil {
		return fmt.Errorf("mark used: %w", err)
	}

	s.log.Info("email verified", "user_id", token.UserID)
	return nil
}
