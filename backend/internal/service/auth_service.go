package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailTaken         = errors.New("email already taken")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrAccountLocked      = errors.New("account is locked, try again later")
)

const (
	maxFailedLoginAttempts = 5
	lockoutDuration        = 15 * time.Minute
)

type AuthService struct {
	userRepo            repository.UserRepository
	verificationService *VerificationService
	log                 *slog.Logger
}

func NewAuthService(
	userRepo repository.UserRepository,
	verificationService *VerificationService,
	log *slog.Logger,
) *AuthService {
	return &AuthService{
		userRepo:            userRepo,
		verificationService: verificationService,
		log:                 log,
	}
}

func (s *AuthService) Register(ctx context.Context, req model.RegisterRequest) (*model.User, error) {
	existing, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.userRepo.CreateWithPassword(ctx, req.Email, req.Name, string(hash))
	if errors.Is(err, repository.ErrUniqueViolation) {
		// users.email is the only unique column CreateWithPassword writes;
		// this is a concurrent Register that passed the check above.
		return nil, ErrEmailTaken
	}
	if err != nil {
		return nil, err
	}

	// Generate a verification token. Email delivery arrives with E-1; until
	// then use `make dev-token` to read it from the database.
	// Note: We don't fail registration if this fails, but we log the failure
	if s.verificationService != nil {
		if _, err := s.verificationService.SendVerification(ctx, user.ID); err != nil {
			s.log.Warn("failed to send verification token", "user_id", user.ID, "error", err)
		}
	}

	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*model.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	// Check if account is locked
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		return nil, ErrAccountLocked
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		// Record failed attempt
		if err := s.userRepo.RecordFailedLogin(ctx, user.ID, maxFailedLoginAttempts, lockoutDuration); err != nil {
			// Still return invalid credentials, but the attempt did not count
			// toward lockout, so make that visible.
			s.log.Warn("failed to record failed login", "user_id", user.ID, "error", err)
		}
		return nil, ErrInvalidCredentials
	}

	// Successful login — reset failed attempts
	if err := s.userRepo.ResetLoginAttempts(ctx, user.ID); err != nil {
		// Don't fail a login that already succeeded.
		s.log.Warn("failed to reset login attempts", "user_id", user.ID, "error", err)
	}

	return user, nil
}
