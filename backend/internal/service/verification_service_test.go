package service

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

const testVerificationToken = "sekret-token-value-123"

// stubVerificationRepo hands out one fixed, valid token.
type stubVerificationRepo struct {
	token *model.VerificationToken
}

func newStubVerificationRepo() *stubVerificationRepo {
	return &stubVerificationRepo{token: &model.VerificationToken{
		ID:        1,
		UserID:    42,
		Token:     testVerificationToken,
		ExpiresAt: time.Now().Add(time.Hour),
	}}
}

func (r *stubVerificationRepo) Create(context.Context, int64) (*model.VerificationToken, error) {
	return r.token, nil
}

func (r *stubVerificationRepo) GetByToken(context.Context, string) (*model.VerificationToken, error) {
	return r.token, nil
}

func (r *stubVerificationRepo) MarkUsed(context.Context, int64) error { return nil }

func (r *stubVerificationRepo) DeleteExpired(context.Context) (int64, error) { return 0, nil }

var _ repository.VerificationRepository = (*stubVerificationRepo)(nil)

// newTestVerificationService logs everything (debug and up) into buf.
func newTestVerificationService(buf *bytes.Buffer) *VerificationService {
	log := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return NewVerificationService(repository.NewMockUserRepo(), newStubVerificationRepo(), log)
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

	// The stub token points at a user that MockUserRepo does not hold.
	userRepo := repository.NewMockUserRepo()
	userRepo.MarkEmailVerifiedFunc = func(context.Context, int64) error { return nil }
	svc := NewVerificationService(userRepo, newStubVerificationRepo(), log)

	if err := svc.Verify(context.Background(), testVerificationToken); err != nil {
		t.Fatalf("Verify() failed: %v", err)
	}
	if strings.Contains(buf.String(), testVerificationToken) {
		t.Errorf("token leaked into logs: %s", buf.String())
	}
}
