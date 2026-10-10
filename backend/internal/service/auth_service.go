package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/template/internal/model"
	"github.com/ssr0016/template/internal/repository"
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
}

func NewAuthService(userRepo repository.UserRepository, verificationService *VerificationService) *AuthService {
	return &AuthService{
		userRepo:            userRepo,
		verificationService: verificationService,
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
	if err != nil {
		return nil, err
	}

	// Send verification email (log token in dev)
	// Note: We don't fail registration if this fails
	if s.verificationService != nil {
		_, _ = s.verificationService.SendVerification(ctx, user.ID)
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
			// Log error but still return invalid credentials
			_ = err
		}
		return nil, ErrInvalidCredentials
	}

	// Successful login — reset failed attempts
	if err := s.userRepo.ResetLoginAttempts(ctx, user.ID); err != nil {
		// Log error but don't fail login
		_ = err
	}

	return user, nil
}
