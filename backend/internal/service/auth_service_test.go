package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

// discardLog is for tests that do not assert on log output.
var discardLog = slog.New(slog.DiscardHandler)

const testPassword = "X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK"

func TestAuthService_Register_Success(t *testing.T) {
	repo := repository.NewMockUserRepo()
	svc := NewAuthService(repo, nil, discardLog)

	user, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "test@example.com",
		Name:     "Test User",
		Password: testPassword,
	})

	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}
	if user == nil {
		t.Fatal("Register() returned nil user")
	}
	if user.Email != "test@example.com" {
		t.Errorf("Email = %s, want test@example.com", user.Email)
	}
	if user.Name != "Test User" {
		t.Errorf("Name = %s, want Test User", user.Name)
	}
	if user.PasswordHash == "" {
		t.Error("PasswordHash is empty")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(testPassword)); err != nil {
		t.Errorf("Password hash doesn't match: %v", err)
	}
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	repo := repository.NewMockUserRepo()
	svc := NewAuthService(repo, nil, discardLog)

	_, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "dup@example.com",
		Name:     "User 1",
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("First Register() failed: %v", err)
	}

	_, err = svc.Register(context.Background(), model.RegisterRequest{
		Email:    "dup@example.com",
		Name:     "User 2",
		Password: testPassword,
	})

	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("Register() error = %v, want ErrEmailTaken", err)
	}
}

func TestAuthService_Register_RepoError(t *testing.T) {
	repo := repository.NewMockUserRepo()
	repo.GetByEmailFunc = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("db error")
	}
	svc := NewAuthService(repo, nil, discardLog)

	_, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "err@example.com",
		Name:     "Err User",
		Password: testPassword,
	})

	if err == nil {
		t.Fatal("Register() expected error, got nil")
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	repo := repository.NewMockUserRepo()
	svc := NewAuthService(repo, nil, discardLog)

	registered, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "login@example.com",
		Name:     "Login User",
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	user, err := svc.Login(context.Background(), "login@example.com", testPassword)
	if err != nil {
		t.Fatalf("Login() failed: %v", err)
	}
	if user.ID != registered.ID {
		t.Errorf("User.ID = %d, want %d", user.ID, registered.ID)
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	repo := repository.NewMockUserRepo()
	svc := NewAuthService(repo, nil, discardLog)

	_, err := svc.Login(context.Background(), "nobody@example.com", testPassword)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	repo := repository.NewMockUserRepo()
	svc := NewAuthService(repo, nil, discardLog)

	_, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "wrong@example.com",
		Name:     "Wrong User",
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	_, err = svc.Login(context.Background(), "wrong@example.com", "wrong-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Login_RepoError(t *testing.T) {
	repo := repository.NewMockUserRepo()
	repo.GetByEmailFunc = func(ctx context.Context, email string) (*model.User, error) {
		return nil, errors.New("db error")
	}
	svc := NewAuthService(repo, nil, discardLog)

	_, err := svc.Login(context.Background(), "err@example.com", testPassword)
	if err == nil {
		t.Fatal("Login() expected error, got nil")
	}
}

// A concurrent Register can pass the GetByEmail check and then lose the insert
// to the unique index; that must read as "email taken", not a server error.
func TestAuthService_Register_LosesEmailRace(t *testing.T) {
	repo := repository.NewMockUserRepo()
	repo.CreateWithPasswordFunc = func(context.Context, string, string, string) (*model.User, error) {
		return nil, fmt.Errorf("scan user: %w", repository.ErrUniqueViolation)
	}
	svc := NewAuthService(repo, nil, discardLog)

	_, err := svc.Register(context.Background(), model.RegisterRequest{
		Email:    "race@example.com",
		Name:     "Racer",
		Password: testPassword,
	})

	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("Register() error = %v, want ErrEmailTaken", err)
	}
}

// Login discards tracking errors so a database hiccup never blocks a login,
// but they must be logged: a failed RecordFailedLogin means the account is
// silently not counting toward lockout.
func TestAuthService_Login_LogsTrackingFailures(t *testing.T) {
	trackingErr := errors.New("connection refused")

	tests := []struct {
		name      string
		password  string
		configure func(*repository.MockUserRepo)
		wantErr   error
		wantMsg   string
	}{
		{"failed login not recorded", "wrong-password",
			func(r *repository.MockUserRepo) {
				r.RecordFailedLoginFunc = func(context.Context, int64, int, time.Duration) error { return trackingErr }
			},
			ErrInvalidCredentials, "failed to record failed login"},
		{"attempt reset failure", testPassword,
			func(r *repository.MockUserRepo) {
				r.ResetLoginAttemptsFunc = func(context.Context, int64) error { return trackingErr }
			},
			nil, "failed to reset login attempts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			repo := repository.NewMockUserRepo()
			svc := NewAuthService(repo, nil, log)

			registered, err := svc.Register(context.Background(), model.RegisterRequest{
				Email: "track@example.com", Name: "Track", Password: testPassword,
			})
			if err != nil {
				t.Fatalf("Register() failed: %v", err)
			}
			tt.configure(repo)

			_, err = svc.Login(context.Background(), "track@example.com", tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Login() error = %v, want %v", err, tt.wantErr)
			}

			var entry map[string]any
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("no JSON log line written: %v (output %q)", err, buf.String())
			}
			if entry["level"] != "WARN" || entry["msg"] != tt.wantMsg {
				t.Errorf("log = level %v msg %v, want WARN %q", entry["level"], entry["msg"], tt.wantMsg)
			}
			if entry["user_id"] != float64(registered.ID) {
				t.Errorf("user_id = %v, want %d", entry["user_id"], registered.ID)
			}
			if strings.Contains(buf.String(), "track@example.com") {
				t.Errorf("email leaked into logs: %s", buf.String())
			}
		})
	}
}
