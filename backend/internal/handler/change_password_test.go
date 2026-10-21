package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/token"
	"github.com/ssr0016/pastoral-reports/backend/internal/config"
	"github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

const (
	cpOld = "Old-Passw0rd-Kd72-qZ"
	cpNew = "New-Passw0rd-Xv91-mT"
)

type cpTx struct{}

func (cpTx) InTransaction(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type cpFixture struct {
	e      *echo.Echo
	users  *repository.MockUserRepo
	audit  *repository.MockAuditRepo
	tokens *token.Manager
	kept   string
	userID int64

	// validAfter is what the real repository stores as users.tokens_valid_after.
	validAfter time.Time
}

const cpFamily = "family-1"

// newCPFixture serves POST /auth/change-password and a /whoami probe behind the
// real Authenticate and RequireRole middleware, so the access cookie can be
// followed across requests and a revoked token is refused as it is in production.
func newCPFixture(t *testing.T) *cpFixture {
	t.Helper()
	f := &cpFixture{users: repository.NewMockUserRepo(), audit: repository.NewMockAuditRepo()}
	f.tokens = token.New(token.Config{Secret: []byte("0123456789abcdef0123456789abcdef"), Issuer: "i", Audience: "a", AccessTTL: time.Minute})
	hash, err := bcrypt.GenerateFromPassword([]byte(cpOld), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	u, err := f.users.CreateWithPassword(context.Background(), "ana@example.com", "Ana", string(hash))
	if err != nil {
		t.Fatal(err)
	}
	f.userID = u.ID
	f.users.GetWithRoleFunc = func(_ context.Context, id int64) (*model.User, error) {
		got, err := f.users.GetByID(context.Background(), id)
		if got != nil {
			got.Role = &model.Role{Name: "user"}
			got.TokensValidAfter = f.validAfter
		}
		return got, err
	}
	f.users.UpdatePasswordKeepSessionFunc = func(_ context.Context, id int64, hash, keep string) (int64, error) {
		f.kept = keep
		f.validAfter = time.Now().Truncate(time.Millisecond)
		return 0, f.users.UpdatePassword(context.Background(), id, hash)
	}

	svc := service.NewAuthService(f.users, nil, nil, f.audit, cpTx{}, slog.New(slog.DiscardHandler)).
		WithTokens(f.tokens, repository.NewMockRefreshRepo(), service.TokenSettings{})
	h := NewAuthHandler(svc, NewCookies(config.AuthConfig{}, false))

	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(middleware.Authenticate(f.tokens))
	e.POST("/test-login", func(c echo.Context) error {
		access, exp, err := f.tokens.Issue(u.ID, cpFamily)
		if err != nil {
			return err
		}
		NewCookies(config.AuthConfig{}, false).SetAccess(c, access, exp)
		return c.NoContent(http.StatusNoContent)
	})
	e.GET("/whoami", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) },
		middleware.RequireAuth(), middleware.RequireRole(f.users, "user"))
	e.POST("/auth/change-password", h.ChangePassword, middleware.RequireAuth())
	f.e = e
	return f
}

func (f *cpFixture) do(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	f.e.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == middleware.AccessCookieName {
			return c
		}
	}
	t.Fatalf("no access cookie in %v", rec.Result().Cookies())
	return nil
}

func (f *cpFixture) signIn(t *testing.T) *http.Cookie {
	t.Helper()
	return sessionCookie(t, f.do(http.MethodPost, "/test-login", "", nil))
}

// cpBody confirms the new password with the same value, as a form would.
func cpBody(current, next string) string { return cpBodyConfirm(current, next, next) }

// cpBodyConfirm is cpBody with an explicit confirmation ("" leaves the field out).
func cpBodyConfirm(current, next, confirm string) string {
	m := map[string]string{"current_password": current, "new_password": next}
	if confirm != "" {
		m["confirm_password"] = confirm
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func details(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var body struct {
		Details map[string]string `json:"details"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return body.Details
}

func TestChangePassword_Success_KeepsThisSignInAndRenewsItsToken(t *testing.T) {
	f := newCPFixture(t)
	old := f.signIn(t)

	rec := f.do(http.MethodPost, "/auth/change-password", cpBody(cpOld, cpNew), old)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (%s)", rec.Code, rec.Body.String())
	}
	if f.kept != cpFamily {
		t.Errorf("kept sign-in = %q, want the caller's %q: the others end, this one must not", f.kept, cpFamily)
	}
	fresh := sessionCookie(t, rec)
	if fresh.Value == old.Value {
		t.Error("the access token was not renewed after the password change")
	}
	if got := f.do(http.MethodGet, "/whoami", "", fresh); got.Code != http.StatusNoContent {
		t.Errorf("the renewed sign-in was signed out: status %d", got.Code)
	}
	if got := f.do(http.MethodGet, "/whoami", "", old); got.Code != http.StatusUnauthorized {
		t.Errorf("the old token still works: status %d", got.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 carried a body: %q", rec.Body.String())
	}
	if len(f.audit.Entries()) != 1 {
		t.Errorf("audit rows = %d, want 1", len(f.audit.Entries()))
	}
}

func TestChangePassword_NoSessionIs401(t *testing.T) {
	f := newCPFixture(t)
	if rec := f.do(http.MethodPost, "/auth/change-password", cpBody(cpOld, cpNew), nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// A wrong current password is a field error, not a 401: the client treats every
// 401 as "your session ended" and signs the user out.
func TestChangePassword_WrongCurrentIs422OnThatField(t *testing.T) {
	f := newCPFixture(t)
	rec := f.do(http.MethodPost, "/auth/change-password", cpBody("not-the-password", cpNew), f.signIn(t))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if got := details(t, rec); got["current_password"] == "" || got["new_password"] != "" {
		t.Errorf("details = %v, want only current_password", got)
	}
	if len(f.audit.Entries()) != 0 {
		t.Error("a failed attempt wrote an audit row")
	}
}

func TestChangePassword_InvalidNewPasswordIs422OnNewPassword(t *testing.T) {
	tests := []struct{ name, next string }{
		{"too weak", "password"},
		{"same as current", cpOld},
		{"over 72 bytes", strings.Repeat("aB3$xY9!", 10)}, // 80 bytes, strong enough
		{"over 72 bytes in multi-byte characters", strings.Repeat("é", 20) + "aB3$xY9!kLm2Q" + strings.Repeat("é", 20)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newCPFixture(t)
			rec := f.do(http.MethodPost, "/auth/change-password", cpBody(cpOld, tt.next), f.signIn(t))
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422 (%s)", rec.Code, rec.Body.String())
			}
			if got := details(t, rec); got["new_password"] == "" {
				t.Errorf("details = %v, want new_password", got)
			}
			if strings.Contains(rec.Body.String(), cpOld) {
				t.Error("the response echoed a password")
			}
		})
	}
}

func TestChangePassword_MissingFieldsAre422(t *testing.T) {
	f := newCPFixture(t)
	rec := f.do(http.MethodPost, "/auth/change-password", `{}`, f.signIn(t))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	got := details(t, rec)
	if got["current_password"] == "" || got["new_password"] == "" {
		t.Errorf("details = %v, want both fields", got)
	}
}

func TestChangePassword_MalformedBodyIs400(t *testing.T) {
	f := newCPFixture(t)
	if rec := f.do(http.MethodPost, "/auth/change-password", "{", f.signIn(t)); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestChangePassword_LockedAccountIs403(t *testing.T) {
	f := newCPFixture(t)
	cookie := f.signIn(t)
	for i := 0; i < 5; i++ {
		f.do(http.MethodPost, "/auth/change-password", cpBody("wrong-password", cpNew), cookie)
	}
	rec := f.do(http.MethodPost, "/auth/change-password", cpBody(cpOld, cpNew), cookie)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 even with the right password (%s)", rec.Code, rec.Body.String())
	}
}

func TestChangePassword_UserGoneIs401(t *testing.T) {
	f := newCPFixture(t)
	cookie := f.signIn(t)
	f.users.GetWithRoleFunc = func(context.Context, int64) (*model.User, error) { return nil, nil }
	if rec := f.do(http.MethodPost, "/auth/change-password", cpBody(cpOld, cpNew), cookie); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestChangePassword_ServerErrorIs500AndHidesTheCause(t *testing.T) {
	f := newCPFixture(t)
	cookie := f.signIn(t)
	f.users.UpdatePasswordKeepSessionFunc = func(context.Context, int64, string, string) (int64, error) {
		return 0, context.DeadlineExceeded
	}
	rec := f.do(http.MethodPost, "/auth/change-password", cpBody(cpOld, cpNew), cookie)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "deadline") {
		t.Errorf("the response leaked the cause: %s", rec.Body.String())
	}
}

func TestChangePassword_ConfirmMismatchIs422AndTouchesNothing(t *testing.T) {
	tests := []struct{ name, confirm, want string }{
		{"missing", "", "is required"},
		{"different", cpNew + "x", "does not match"},
		{"case-only difference", strings.ToUpper(cpNew), "does not match"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newCPFixture(t)
			// A wrong current password would be a 422 on current_password and a lockout strike;
			// the confirm check must answer first and spend no strike.
			rec := f.do(http.MethodPost, "/auth/change-password", cpBodyConfirm("not-the-password", cpNew, tt.confirm), f.signIn(t))
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body %s", rec.Code, rec.Body.String())
			}
			d := details(t, rec)
			if d["confirm_password"] != tt.want {
				t.Errorf("details[confirm_password] = %q, want %q (all: %v)", d["confirm_password"], tt.want, d)
			}
			if _, ok := d["current_password"]; ok {
				t.Errorf("current_password was checked before the confirmation: %v", d)
			}
		})
	}
}
