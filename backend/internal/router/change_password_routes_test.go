package router

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/authimpl"
	authrest "github.com/ssr0016/pastoral-reports/backend/internal/auth/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/token"
	"github.com/ssr0016/pastoral-reports/backend/internal/config"
	reportrest "github.com/ssr0016/pastoral-reports/backend/internal/report/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

const (
	routeOldPassword = "Old-Passw0rd-Kd72-qZ"
	routeNewPassword = "New-Passw0rd-Xv91-mT"
)

// changePasswordServer wires the real routes with the real auth service over mocks.
// A request is logged in as user 1 when it carries X-Test-User.
func changePasswordServer(t *testing.T, write func() echo.MiddlewareFunc) *echo.Echo {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(routeOldPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	users := userimpl.NewMockStore()
	if _, err := users.CreateWithPassword(context.Background(), "ana@example.com", "Ana", string(hash)); err != nil {
		t.Fatal(err)
	}
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*user.User, error) {
		u, err := users.GetByID(context.Background(), id)
		if u != nil {
			u.Role = &accesscontrol.Role{Name: "user"}
		}
		return u, err
	}
	svc := authimpl.NewService(users, nil, nil, auditimpl.NewMockStore(), runTx{}, slog.New(slog.DiscardHandler), token.New(token.Config{Secret: []byte("0123456789abcdef0123456789abcdef"), Issuer: "i", Audience: "a", AccessTTL: time.Minute}),
		authimpl.NewMockRefreshStore(), authimpl.TokenSettings{})

	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if id, err := strconv.ParseInt(c.Request().Header.Get("X-Test-User"), 10, 64); err == nil {
				c.SetRequest(c.Request().WithContext(request.WithUser(c.Request().Context(), request.User{ID: id})))
			}
			return next(c)
		}
	})
	Setup(e, noAuthn, users, authrest.NewAuthHandler(svc, authrest.NewCookies(config.AuthConfig{}, false)), nil, nil, nil, nil,
		authrest.NewVerificationHandler(nil), authrest.NewPasswordResetHandler(nil),
		reportrest.NewReportHandler(okReportSvc{}), nil, nil,
		Limiters{Global: noLimit, Read: noLimit, Write: write()},
	)
	return e
}

// changePassword sends routeNewPassword as the new password: no test here varies it.
func changePassword(e *echo.Echo, current, user string) int {
	body := `{"current_password":"` + current + `","new_password":"` + routeNewPassword + `","confirm_password":"` + routeNewPassword + `"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if user != "" {
		req.Header.Set("X-Test-User", user)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Code
}

func TestChangePasswordRoute_NeedsASession(t *testing.T) {
	e := changePasswordServer(t, func() echo.MiddlewareFunc { return noLimit })
	if got := changePassword(e, routeOldPassword, ""); got != http.StatusUnauthorized {
		t.Errorf("signed out: status = %d, want 401", got)
	}
	if got := changePassword(e, routeOldPassword, "1"); got != http.StatusNoContent {
		t.Errorf("signed in: status = %d, want 204", got)
	}
}

// It shares the per-user write bucket with the reports writes, so a signed-in
// caller cannot hammer it; a signed-out caller never reaches the limiter.
func TestChangePasswordRoute_IsRateLimitedPerUser(t *testing.T) {
	e := changePasswordServer(t, userLimit(2))
	for i := 0; i < 5; i++ {
		if got := changePassword(e, routeOldPassword, ""); got != http.StatusUnauthorized {
			t.Fatalf("signed-out #%d: status = %d, want 401 (never 429)", i, got)
		}
	}
	for i := 0; i < 2; i++ {
		if got := changePassword(e, "wrong-password", "1"); got != http.StatusUnprocessableEntity {
			t.Fatalf("attempt #%d: status = %d, want 422", i, got)
		}
	}
	if got := changePassword(e, "wrong-password", "1"); got != http.StatusTooManyRequests {
		t.Errorf("attempt #3: status = %d, want 429", got)
	}
}
