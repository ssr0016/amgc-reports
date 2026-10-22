package authimpl

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

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
	"golang.org/x/crypto/bcrypt"
)

// discardLog is for tests that do not assert on log output.
var discardLog = slog.New(slog.DiscardHandler)

const testPassword = "X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK"

// seedUser stores an account whose password is testPassword. bcrypt.MinCost
// keeps the login tests fast; the cost is not what they test.
func seedUser(t *testing.T, repo *userimpl.MockStore, email, name string) *user.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	usr, err := repo.CreateWithPassword(context.Background(), email, name, string(hash))
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return usr
}

func TestAuthService_Login_Success(t *testing.T) {
	repo := userimpl.NewMockStore()
	svc := newService(repo, nil, nil, auditimpl.NewMockStore(), nil, discardLog)

	registered := seedUser(t, repo, "login@example.com", "Login User")

	user, err := svc.Login(context.Background(), "login@example.com", testPassword, "ip")
	if err != nil {
		t.Fatalf("Login() failed: %v", err)
	}
	if user.ID != registered.ID {
		t.Errorf("User.ID = %d, want %d", user.ID, registered.ID)
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	repo := userimpl.NewMockStore()
	svc := newService(repo, nil, nil, auditimpl.NewMockStore(), nil, discardLog)

	_, err := svc.Login(context.Background(), "nobody@example.com", testPassword, "ip")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("Login() error = %v, want auth.ErrInvalidCredentials", err)
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	repo := userimpl.NewMockStore()
	svc := newService(repo, nil, nil, auditimpl.NewMockStore(), nil, discardLog)

	seedUser(t, repo, "wrong@example.com", "Wrong User")

	_, err := svc.Login(context.Background(), "wrong@example.com", "wrong-password", "ip")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("Login() error = %v, want auth.ErrInvalidCredentials", err)
	}
}

func TestAuthService_Login_RepoError(t *testing.T) {
	repo := userimpl.NewMockStore()
	repo.GetByEmailFunc = func(ctx context.Context, email string) (*user.User, error) {
		return nil, errors.New("db error")
	}
	svc := newService(repo, nil, nil, auditimpl.NewMockStore(), nil, discardLog)

	_, err := svc.Login(context.Background(), "err@example.com", testPassword, "ip")
	if err == nil {
		t.Fatal("Login() expected error, got nil")
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
		configure func(*userimpl.MockStore)
		wantErr   error
		wantMsg   string
	}{
		{"failed login not recorded", "wrong-password",
			func(r *userimpl.MockStore) {
				r.RecordFailedLoginFunc = func(context.Context, int64, int, time.Duration) error { return trackingErr }
			},
			auth.ErrInvalidCredentials, "failed to record failed login"},
		{"attempt reset failure", testPassword,
			func(r *userimpl.MockStore) {
				r.ResetLoginAttemptsFunc = func(context.Context, int64) error { return trackingErr }
			},
			nil, "failed to reset login attempts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			repo := userimpl.NewMockStore()
			svc := newService(repo, nil, nil, auditimpl.NewMockStore(), nil, log)

			registered := seedUser(t, repo, "track@example.com", "Track")
			tt.configure(repo)

			_, err := svc.Login(context.Background(), "track@example.com", tt.password, "ip")
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

// Login checks the status only after the password, so the three messages can never
// be used to find out which emails have an account (D-A10).
func TestAuthService_Login_RefusesANonActiveAccountAfterThePassword(t *testing.T) {
	tests := []struct {
		status string
		want   error
	}{
		{user.UserStatusInactive, auth.ErrAccountInactive},
		{user.UserStatusDisciplinary, auth.ErrAccountUnderReview},
		{user.UserStatusDeceased, auth.ErrAccountClosed},
		{"retired", auth.ErrAccountInactive}, // a value this code does not know is refused, never let through
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			repo := userimpl.NewMockStore()
			svc := newService(repo, nil, nil, auditimpl.NewMockStore(), nil, discardLog)
			u := seedUser(t, repo, "s@example.com", "S")
			u.Status = tt.status
			repo.ResetLoginAttemptsFunc = func(context.Context, int64) error {
				t.Error("a refused login must not reset the failed-attempt counter")
				return nil
			}

			_, err := svc.Login(context.Background(), "s@example.com", testPassword, "ip")

			if !errors.Is(err, tt.want) {
				t.Errorf("Login() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestAuthService_Login_WrongPasswordOnANonActiveAccountIsGeneric(t *testing.T) {
	repo := userimpl.NewMockStore()
	svc := newService(repo, nil, nil, auditimpl.NewMockStore(), nil, discardLog)
	u := seedUser(t, repo, "s@example.com", "S")
	u.Status = user.UserStatusDeceased
	recorded := false
	repo.RecordFailedLoginFunc = func(context.Context, int64, int, time.Duration) error { recorded = true; return nil }

	_, err := svc.Login(context.Background(), "s@example.com", "wrong-password", "ip")

	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("Login() error = %v, want auth.ErrInvalidCredentials (the status must not leak)", err)
	}
	if !recorded {
		t.Error("the wrong attempt did not count toward lockout")
	}
}

// A deleted user looks exactly like an unknown email (D-A11), even with the right
// password and a lock in place: no lockout message, no failed attempt recorded.
func TestAuthService_Login_DeletedUserIsAnUnknownEmail(t *testing.T) {
	repo := userimpl.NewMockStore()
	svc := newService(repo, nil, nil, auditimpl.NewMockStore(), nil, discardLog)
	u := seedUser(t, repo, "d@example.com", "D")
	now := time.Now()
	u.DeletedAt = &now
	locked := now.Add(time.Hour)
	u.LockedUntil = &locked
	repo.RecordFailedLoginFunc = func(context.Context, int64, int, time.Duration) error {
		t.Error("a deleted user's login must not record a failed attempt")
		return nil
	}

	for _, pw := range []string{testPassword, "wrong-password"} {
		if _, err := svc.Login(context.Background(), "d@example.com", pw, "ip"); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Errorf("Login(%q) error = %v, want auth.ErrInvalidCredentials", pw, err)
		}
	}
}

func TestAuthService_Me_NonActiveIsUnauthorized(t *testing.T) {
	users := userimpl.NewMockStore()
	users.GetWithRoleFunc = func(context.Context, int64) (*user.User, error) {
		return &user.User{ID: 1, Name: "Ed", Status: user.UserStatusInactive}, nil
	}
	svc := newService(users, stubRoles{}, nil, nil, nil, discardLog)

	_, err := svc.Me(context.Background(), 1)

	if !errors.Is(err, auth.ErrUnauthorized) {
		t.Errorf("Me() error = %v, want auth.ErrUnauthorized", err)
	}
}

func TestAuthService_Me(t *testing.T) {
	users := userimpl.NewMockStore()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*user.User, error) {
		switch id {
		case 1:
			return &user.User{ID: 1, Name: "Ed", Email: "ed@example.org", RoleID: 20, Role: &accesscontrol.Role{Name: "editor"}}, nil
		case 2: // no role assigned
			return &user.User{ID: 2, Name: "Nora", Email: "nora@example.org"}, nil
		case 3: // profile set by the admin
			a, c := "Metro", "AMGC 1"
			return &user.User{ID: 3, Name: "Pia", Email: "pia@example.org", AreaAssignment: &a, ChurchName: &c}, nil
		}
		return nil, nil
	}
	roles := stubRoles{perms: map[int64][]string{20: {"reports:approve", "reports:read"}}}
	svc := newService(users, roles, nil, nil, nil, discardLog)
	ctx := context.Background()

	t.Run("role and permissions", func(t *testing.T) {
		got, err := svc.Me(ctx, 1)
		if err != nil {
			t.Fatalf("Me() failed: %v", err)
		}
		want := auth.MeResponse{UserID: 1, Name: "Ed", Email: "ed@example.org", Role: "editor",
			Permissions: []string{"reports:approve", "reports:read"}}
		if fmt.Sprint(*got) != fmt.Sprint(want) {
			t.Errorf("Me() = %+v, want %+v", *got, want)
		}
	})

	t.Run("profile fields come through; unset ones are empty strings", func(t *testing.T) {
		got, err := svc.Me(ctx, 3)
		if err != nil {
			t.Fatalf("Me() failed: %v", err)
		}
		if got.AreaAssignment != "Metro" || got.ChurchName != "AMGC 1" {
			t.Errorf("Me() profile = %q, %q", got.AreaAssignment, got.ChurchName)
		}
		none, err := svc.Me(ctx, 2)
		if err != nil {
			t.Fatalf("Me() failed: %v", err)
		}
		if none.AreaAssignment != "" || none.ChurchName != "" {
			t.Errorf("unset profile = %q, %q, want empty", none.AreaAssignment, none.ChurchName)
		}
	})

	t.Run("no role gives an empty role and a non-nil empty list", func(t *testing.T) {
		got, err := svc.Me(ctx, 2)
		if err != nil {
			t.Fatalf("Me() failed: %v", err)
		}
		if got.Role != "" || got.Permissions == nil || len(got.Permissions) != 0 {
			t.Errorf("Me() = %+v, want empty role and [] permissions", *got)
		}
	})

	t.Run("a user that no longer exists is auth.ErrUnauthorized", func(t *testing.T) {
		if _, err := svc.Me(ctx, 99); !errors.Is(err, auth.ErrUnauthorized) {
			t.Errorf("Me() error = %v, want auth.ErrUnauthorized", err)
		}
	})

	t.Run("repository errors are returned, not hidden as 401", func(t *testing.T) {
		boom := errors.New("db down")
		users.GetWithRoleFunc = func(context.Context, int64) (*user.User, error) { return nil, boom }
		if _, err := svc.Me(ctx, 1); !errors.Is(err, boom) || errors.Is(err, auth.ErrUnauthorized) {
			t.Errorf("Me() error = %v, want the repository error", err)
		}
	})
}
