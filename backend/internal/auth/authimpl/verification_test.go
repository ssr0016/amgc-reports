package authimpl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

const testVerificationToken = "sekret-token-value-123"

// stubVerificationRepo hands out one fixed, valid token.
type stubVerificationRepo struct {
	token       *auth.VerificationToken
	createErr   error
	markUsedErr error
}

func newStubVerificationRepo() *stubVerificationRepo {
	return &stubVerificationRepo{token: &auth.VerificationToken{
		ID:        1,
		UserID:    42,
		Token:     testVerificationToken,
		ExpiresAt: time.Now().Add(time.Hour),
	}}
}

func (r *stubVerificationRepo) Create(context.Context, int64) (*auth.VerificationToken, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	return r.token, nil
}

func (r *stubVerificationRepo) GetByToken(context.Context, string) (*auth.VerificationToken, error) {
	return r.token, nil
}

func (r *stubVerificationRepo) MarkUsed(context.Context, int64) error { return r.markUsedErr }

func (r *stubVerificationRepo) DeleteExpired(context.Context) (int64, error) { return 0, nil }

var _ auth.VerificationStore = (*stubVerificationRepo)(nil)

// newTestVerificationService logs everything (debug and up) into buf.
func newTestVerificationService(buf *bytes.Buffer) auth.VerificationService {
	log := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return NewVerificationService(userimpl.NewMockStore(), newStubVerificationRepo(), log)
}

func TestVerificationService_SendVerification_DoesNotLogToken(t *testing.T) {
	var buf bytes.Buffer
	svc := newTestVerificationService(&buf)

	got, err := svc.SendVerification(context.Background(), 42)
	if err != nil {
		t.Fatalf("SendVerification() failed: %v", err)
	}
	if got != testVerificationToken {
		t.Errorf("returned token = %q, want %q", got, testVerificationToken)
	}

	if strings.Contains(buf.String(), testVerificationToken) {
		t.Errorf("token leaked into logs: %s", buf.String())
	}

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("decode log line %q: %v", buf.String(), err)
	}
	if entry["msg"] != "verification token generated" {
		t.Errorf("msg = %v, want %q", entry["msg"], "verification token generated")
	}
	if entry["user_id"] != float64(42) {
		t.Errorf("user_id = %v, want 42", entry["user_id"])
	}
	if _, ok := entry["expires_at"]; !ok {
		t.Error("expires_at missing from log entry")
	}
}

func TestVerificationService_Verify_DoesNotLogToken(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// The stub token points at a user that the mock user store does not hold.
	userRepo := userimpl.NewMockStore()
	userRepo.MarkEmailVerifiedFunc = func(context.Context, int64) error { return nil }
	svc := NewVerificationService(userRepo, newStubVerificationRepo(), log)

	if err := svc.Verify(context.Background(), testVerificationToken); err != nil {
		t.Fatalf("Verify() failed: %v", err)
	}
	if strings.Contains(buf.String(), testVerificationToken) {
		t.Errorf("token leaked into logs: %s", buf.String())
	}
}

// The token is claimed before the email is marked verified: a request that
// loses the claim must not touch the user.
func TestVerificationService_Verify_LostClaimDoesNotVerifyEmail(t *testing.T) {
	users := userimpl.NewMockStore()
	verified := false
	users.MarkEmailVerifiedFunc = func(context.Context, int64) error {
		verified = true
		return nil
	}
	tokens := newStubVerificationRepo()
	tokens.markUsedErr = database.ErrNotFound
	svc := NewVerificationService(users, tokens, discardLog)

	err := svc.Verify(context.Background(), testVerificationToken)

	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Errorf("Verify() error = %v, want auth.ErrInvalidToken", err)
	}
	if verified {
		t.Error("email was verified although the token claim was lost")
	}
}

func TestVerificationService_Verify_ClaimFailureStaysAnError(t *testing.T) {
	tokens := newStubVerificationRepo()
	tokens.markUsedErr = errors.New("connection refused")
	svc := NewVerificationService(userimpl.NewMockStore(), tokens, discardLog)

	err := svc.Verify(context.Background(), testVerificationToken)

	if err == nil || errors.Is(err, auth.ErrInvalidToken) {
		t.Errorf("Verify() error = %v, want a non-token error", err)
	}
}
