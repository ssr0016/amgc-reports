// Package auth is sign-in and the session: login, refresh tokens, password
// change, email verification and password reset. Callers depend on the
// interfaces here; authimpl implements them and rest serves them.
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountLocked      = errors.New("account is locked, try again later")
	// The account is not active (D-A10). Only returned after the password was right,
	// so they cannot be used to find out which emails have an account.
	ErrAccountInactive    = errors.New("account is inactive")
	ErrAccountUnderReview = errors.New("account is under review")
	ErrAccountClosed      = errors.New("account is closed")
	// ErrPasswordUnchanged: the new password equals the current one.
	ErrPasswordUnchanged = errors.New("new password must differ from the current one")
	// ErrInvalidRefresh is a refresh token that cannot be used: unknown, expired,
	// idle too long, already rotated, or revoked.
	ErrInvalidRefresh = errors.New("invalid refresh token")
	// ErrRefreshReplayed is a rotated refresh token presented again inside
	// refreshGrace: almost certainly a second tab that read the cookie before the
	// first tab's new one arrived. It is refused, but the sign-in is alive and the
	// caller must not clear the cookies, which by now hold the winner's new tokens.
	ErrRefreshReplayed   = errors.New("refresh token already used")
	ErrInvalidToken      = errors.New("invalid or expired token")
	ErrInvalidResetToken = errors.New("invalid or expired reset token")
	// ErrUnauthorized is returned when the caller's account no longer exists.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrPasswordTooLong: bcrypt reads 72 bytes and no more, so a longer password
	// would be silently cut. It counts bytes, not characters.
	ErrPasswordTooLong = errors.New("password must be at most 72 bytes")
)

// MaxPasswordBytes is the longest password bcrypt reads in full.
const MaxPasswordBytes = 72

// TokenPair is what a sign-in or a refresh hands to the handler, which puts it
// in cookies.
type TokenPair struct {
	Access         string
	AccessExpires  time.Time
	Refresh        string
	RefreshExpires time.Time
	SessionID      string
}

// Service signs users in and keeps their sessions: login, the access and
// refresh tokens, Me and ChangePassword.
type Service interface {
	Me(ctx context.Context, userID int64) (*MeResponse, error)
	Login(ctx context.Context, email, password string) (*user.User, error)
	ChangePassword(ctx context.Context, userID int64, ip, keepFamily, current, next string) error
	StartSession(ctx context.Context, userID int64, ip, userAgent string) (*TokenPair, error)
	RenewAccess(userID int64, family string) (string, time.Time, error)
	Refresh(ctx context.Context, raw, ip, userAgent string) (*TokenPair, error)
	EndSession(ctx context.Context, raw string) error
}

// VerificationService issues and redeems email verification tokens.
type VerificationService interface {
	SendVerification(ctx context.Context, userID int64) (string, error)
	Verify(ctx context.Context, tokenStr string) error
}

// PasswordResetService issues and redeems password reset tokens.
type PasswordResetService interface {
	RequestReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, tokenStr, newPassword string) error
}

// RefreshStore defines the interface for refresh-token access. Tokens
// are found by the hash of the token (token.Hash), never by the token.
type RefreshStore interface {
	Create(ctx context.Context, t NewRefreshToken) (int64, error)
	// UserIDByHash is the owner of a token, read without a lock, so a caller can
	// lock the user before the token. found is false for an unknown token.
	UserIDByHash(ctx context.Context, hash string) (userID int64, found bool, err error)
	// GetForUpdate returns the token and locks its row until the transaction in ctx
	// ends, so two refreshes of one token run one after the other. (nil, nil) when
	// there is no such token.
	GetForUpdate(ctx context.Context, hash string) (*RefreshToken, error)
	// MarkReplaced revokes a token that has just been rotated and links its successor.
	MarkReplaced(ctx context.Context, id, replacedBy int64, at time.Time) error
	// RevokeFamily ends a sign-in: every live token of the family.
	RevokeFamily(ctx context.Context, familyID string, at time.Time) error
}

// VerificationStore defines the interface for verification token access.
type VerificationStore interface {
	Create(ctx context.Context, userID int64) (*VerificationToken, error)
	GetByToken(ctx context.Context, token string) (*VerificationToken, error)
	MarkUsed(ctx context.Context, id int64) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// PasswordResetStore defines the interface for password reset token access.
type PasswordResetStore interface {
	Create(ctx context.Context, userID int64) (*PasswordReset, error)
	GetByToken(ctx context.Context, token string) (*PasswordReset, error)
	MarkUsed(ctx context.Context, id int64) error
	DeleteExpired(ctx context.Context) (int64, error)
}
