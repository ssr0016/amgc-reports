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
	roleRepo            repository.RoleRepository // for Me; nil in callers that never call it
	verificationService *VerificationService
	log                 *slog.Logger
}

func NewAuthService(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	verificationService *VerificationService,
	log *slog.Logger,
) *AuthService {
	return &AuthService{
		userRepo:            userRepo,
		roleRepo:            roleRepo,
		verificationService: verificationService,
		log:                 log,
	}
}

// Me returns the user's name, email, role and permissions, read live from the
// database (a role change shows on the next call). A user that no longer exists
// is ErrUnauthorized, so a session outliving its user stops working.
func (s *AuthService) Me(ctx context.Context, userID int64) (*model.MeResponse, error) {
	user, err := s.userRepo.GetWithRole(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, ErrUnauthorized
	}
	me := &model.MeResponse{
		UserID: user.ID, Name: user.Name, Email: user.Email, Permissions: []string{},
		Worker: deref(user.Worker), AreaAssignment: deref(user.AreaAssignment), ChurchName: deref(user.ChurchName),
	}
	if user.Role != nil {
		me.Role = user.Role.Name
	}
	if user.RoleID != 0 {
		perms, err := s.roleRepo.GetPermissions(ctx, user.RoleID)
		if err != nil {
			return nil, fmt.Errorf("get permissions: %w", err)
		}
		for _, p := range perms {
			me.Permissions = append(me.Permissions, p.Name)
		}
	}
	return me, nil
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

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
