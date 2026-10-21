package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/auth/token"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

// ErrInvalidRefresh is a refresh token that cannot be used: unknown, expired,
// idle too long, already rotated, or revoked.
var ErrInvalidRefresh = errors.New("invalid refresh token")

// ErrRefreshReplayed is a rotated refresh token presented again inside
// refreshGrace: almost certainly a second tab that read the cookie before the
// first tab's new one arrived. It is refused, but the sign-in is alive and the
// caller must not clear the cookies, which by now hold the winner's new tokens.
var ErrRefreshReplayed = errors.New("refresh token already used")

// refreshGrace is how long after a rotation the old token is only refused. Two
// tabs sharing one cookie jar can send the same token within moments of each
// other; that is not theft, so the family is kept. Later reuse revokes it.
const refreshGrace = 10 * time.Second

// refreshBytes is the size of a refresh token before encoding: 256 bits.
const refreshBytes = 32

// TokenSettings are the sign-in lifetimes (D5).
type TokenSettings struct {
	RefreshTTL     time.Duration // absolute, from sign-in
	RefreshIdleTTL time.Duration // longest gap between refreshes
}

// TokenPair is what a sign-in or a refresh hands to the handler, which puts it
// in cookies.
type TokenPair struct {
	Access         string
	AccessExpires  time.Time
	Refresh        string
	RefreshExpires time.Time
	SessionID      string
}

// WithTokens enables sign-in sessions. Services built without it (most tests)
// cannot start or refresh one.
func (s *AuthService) WithTokens(issuer *token.Manager, refresh repository.RefreshTokenRepository, set TokenSettings) *AuthService {
	s.tokens, s.refresh, s.tokenSet = issuer, refresh, set
	return s
}

// StartSession opens a sign-in for a user whose credentials Login has accepted:
// a new family with its first refresh token, and an access token for it.
func (s *AuthService) StartSession(ctx context.Context, userID int64, ip, userAgent string) (*TokenPair, error) {
	family, err := token.Random(16)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return s.issuePair(ctx, userID, family, now.Add(s.tokenSet.RefreshTTL), now, ip, userAgent)
}

// issuePair stores a new refresh token in family and signs an access token.
func (s *AuthService) issuePair(ctx context.Context, userID int64, family string, expires, now time.Time, ip, ua string) (*TokenPair, error) {
	refresh, err := token.Random(refreshBytes)
	if err != nil {
		return nil, err
	}
	if _, err := s.refresh.Create(ctx, newRefresh(userID, family, refresh, expires, now, ip, ua)); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}
	return s.pair(userID, family, refresh, expires)
}

func newRefresh(userID int64, family, refresh string, expires, now time.Time, ip, ua string) model.NewRefreshToken {
	return model.NewRefreshToken{
		UserID: userID, FamilyID: family, Hash: token.Hash(refresh),
		ExpiresAt: expires, Now: now, IP: ip, UserAgent: ua,
	}
}

func (s *AuthService) pair(userID int64, family, refresh string, refreshExpires time.Time) (*TokenPair, error) {
	access, exp, err := s.tokens.Issue(userID, family)
	if err != nil {
		return nil, err
	}
	return &TokenPair{Access: access, AccessExpires: exp, Refresh: refresh, RefreshExpires: refreshExpires, SessionID: family}, nil
}

// RenewAccess signs a new access token for a sign-in that stays (after a
// password change, which moves tokens_valid_after past the old one).
func (s *AuthService) RenewAccess(userID int64, family string) (string, time.Time, error) {
	return s.tokens.Issue(userID, family)
}

// Refresh trades a refresh token for a new pair. The old token is retired; the
// new one lives in the same family and inherits its absolute expiry.
//
// The outcome is decided inside the transaction but returned after it, so that
// a revocation (reuse, an account that may no longer sign in) is committed
// rather than rolled back with the error.
func (s *AuthService) Refresh(ctx context.Context, raw, ip, userAgent string) (*TokenPair, error) {
	var (
		pair    *TokenPair
		outcome error
	)
	err := s.tx.InTransaction(ctx, func(ctx context.Context) error {
		var err error
		pair, outcome, err = s.rotate(ctx, raw, ip, userAgent)
		return err
	})
	if err != nil {
		return nil, err
	}
	if outcome != nil {
		return nil, outcome
	}
	return pair, nil
}

// rotate is the body of Refresh's transaction. outcome is the verdict on the
// token (nil: rotated); err is a failure of the store, which rolls back.
func (s *AuthService) rotate(ctx context.Context, raw, ip, userAgent string) (pair *TokenPair, outcome, err error) {
	now := time.Now()
	hash := token.Hash(raw)

	if outcome, err := s.lockOwner(ctx, hash); outcome != nil || err != nil {
		return nil, outcome, err
	}
	old, err := s.refresh.GetForUpdate(ctx, hash) // read again: it may have changed while we waited
	if err != nil || old == nil {
		return nil, ErrInvalidRefresh, err
	}
	if old.RevokedAt != nil {
		outcome, err = s.handleSpent(ctx, old, now, ip)
		return nil, outcome, err
	}
	if now.After(old.ExpiresAt) || now.Sub(old.LastUsedAt) > s.tokenSet.RefreshIdleTTL {
		return nil, ErrInvalidRefresh, s.refresh.RevokeFamily(ctx, old.FamilyID, now)
	}

	user, err := s.userRepo.GetByID(ctx, old.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil || !user.CanSignIn() {
		return nil, ErrUnauthorized, s.refresh.RevokeFamily(ctx, old.FamilyID, now)
	}

	next, err := token.Random(refreshBytes)
	if err != nil {
		return nil, nil, err
	}
	newID, err := s.refresh.Create(ctx, newRefresh(old.UserID, old.FamilyID, next, old.ExpiresAt, now, ip, userAgent))
	if err != nil {
		return nil, nil, fmt.Errorf("store refresh token: %w", err)
	}
	if err := s.refresh.MarkReplaced(ctx, old.ID, newID, now); err != nil {
		return nil, nil, err
	}
	pair, err = s.pair(old.UserID, old.FamilyID, next, old.ExpiresAt)
	return pair, nil, err
}

// lockOwner locks the user a refresh token belongs to, before the caller locks the
// token. Lock order: the user, then the token. Revocations update the users row
// first, so one of them either waits for this rotation and then revokes the token
// it creates, or finishes first and the rotation finds the family revoked. Without
// the user lock a revocation could miss the new token and leave a sign-in alive.
// outcome is ErrInvalidRefresh for an unknown token or a deleted user; err is the store's.
func (s *AuthService) lockOwner(ctx context.Context, hash string) (outcome, err error) {
	owner, found, err := s.refresh.UserIDByHash(ctx, hash)
	if err != nil || !found {
		return ErrInvalidRefresh, err
	}
	switch err := s.userRepo.LockUser(ctx, owner); {
	case errors.Is(err, repository.ErrNotFound):
		return ErrInvalidRefresh, nil
	case err != nil:
		return nil, err
	}
	return nil, nil
}

// handleSpent judges a refresh token that is already revoked. A token rotated
// away moments ago is only replayed (ErrRefreshReplayed). One rotated away long
// ago and back means it was copied: end the whole sign-in. Any other revoked
// token (logout, password change) is plainly invalid. err is the store's.
func (s *AuthService) handleSpent(ctx context.Context, old *model.RefreshToken, now time.Time, ip string) (outcome, err error) {
	if old.ReplacedBy == nil {
		return ErrInvalidRefresh, nil
	}
	if now.Sub(*old.RevokedAt) <= refreshGrace {
		return ErrRefreshReplayed, nil
	}
	s.log.Warn("refresh token reuse, family revoked", "user_id", old.UserID, "family", old.FamilyID, "ip", ip)
	return ErrInvalidRefresh, s.refresh.RevokeFamily(ctx, old.FamilyID, now)
}

// EndSession signs out: it revokes the family of a refresh token. An unknown
// token is not an error, so logging out twice, or after the token expired, works.
func (s *AuthService) EndSession(ctx context.Context, raw string) error {
	return s.tx.InTransaction(ctx, func(ctx context.Context) error {
		t, err := s.refresh.GetForUpdate(ctx, token.Hash(raw))
		if err != nil || t == nil {
			return err
		}
		return s.refresh.RevokeFamily(ctx, t.FamilyID, time.Now())
	})
}
