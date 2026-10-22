package rest

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/authimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/token"
	"github.com/ssr0016/pastoral-reports/backend/internal/config"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

const loginPassword = "X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK"

// serveLogin signs in as a stored account with the given status and lock time.
func serveLogin(t *testing.T, status string, lockedUntil *time.Time, password string) *httptest.ResponseRecorder {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(loginPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	users := userimpl.NewMockStore()
	u, err := users.CreateWithPassword(context.Background(), "s@example.com", "S", string(hash))
	if err != nil {
		t.Fatal(err)
	}
	u.Status, u.LockedUntil = status, lockedUntil

	svc := authimpl.NewService(users, nil, nil, auditimpl.NewMockStore(), cpTx{}, slog.New(slog.DiscardHandler), token.New(token.Config{Secret: []byte("0123456789abcdef0123456789abcdef"), Issuer: "i", Audience: "a", AccessTTL: time.Minute}),
		authimpl.NewMockRefreshStore(), authimpl.TokenSettings{RefreshTTL: time.Hour})
	h := NewAuthHandler(svc, NewCookies(config.AuthConfig{}, false))
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.POST("/auth/login", h.Login)

	body := `{"email":"s@example.com","password":"` + password + `"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestAuthHandler_Login_StatusMessages(t *testing.T) {
	future := time.Now().Add(time.Hour)
	tests := []struct {
		name     string
		status   string
		locked   *time.Time
		password string
		code     int
		message  string
	}{
		{"active signs in", user.UserStatusActive, nil, loginPassword, http.StatusOK, ""},
		{"inactive", user.UserStatusInactive, nil, loginPassword, http.StatusForbidden, "Your account is inactive. Contact your admin."},
		{"disciplinary", user.UserStatusDisciplinary, nil, loginPassword, http.StatusForbidden, "Your account is under review. Contact your admin."},
		{"deceased", user.UserStatusDeceased, nil, loginPassword, http.StatusForbidden, "This account is closed."},
		{"wrong password on a closed account stays generic", user.UserStatusDeceased, nil, "wrong-password", http.StatusUnauthorized, "invalid email or password"},
		{"locked", user.UserStatusActive, &future, loginPassword, http.StatusForbidden, "Account locked. Try again later."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serveLogin(t, tt.status, tt.locked, tt.password)
			if rec.Code != tt.code {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.code, rec.Body.String())
			}
			if tt.message != "" && !strings.Contains(rec.Body.String(), tt.message) {
				t.Errorf("body = %s, want it to contain %q", rec.Body.String(), tt.message)
			}
		})
	}
}

// A successful login sets both cookies, HttpOnly, with the refresh token confined to the auth routes.
func TestAuthHandler_Login_SetsTheAuthCookies(t *testing.T) {
	rec := serveLogin(t, user.UserStatusActive, nil, loginPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	byName := map[string]*http.Cookie{}
	for _, c := range rec.Result().Cookies() {
		byName[c.Name] = c
	}
	access, refresh := byName["access_token"], byName["refresh_token"]
	if access == nil || refresh == nil {
		t.Fatalf("cookies = %v, want access_token and refresh_token", rec.Result().Cookies())
	}
	for _, c := range []*http.Cookie{access, refresh} {
		if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Value == "" {
			t.Errorf("%s: HttpOnly=%v SameSite=%v empty=%v", c.Name, c.HttpOnly, c.SameSite, c.Value == "")
		}
	}
	if access.Path != "/" || refresh.Path != "/api/v1/auth" {
		t.Errorf("paths = %q and %q", access.Path, refresh.Path)
	}
	if strings.Contains(rec.Body.String(), refresh.Value) || strings.Contains(rec.Body.String(), access.Value) {
		t.Error("a token was echoed in the response body")
	}
}

// A refused login sets no cookie.
func TestAuthHandler_Login_RefusedSetsNoCookie(t *testing.T) {
	rec := serveLogin(t, user.UserStatusInactive, nil, loginPassword)
	if got := rec.Result().Cookies(); len(got) != 0 {
		t.Errorf("cookies = %v, want none", got)
	}
}
