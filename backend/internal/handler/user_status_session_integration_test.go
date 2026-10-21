//go:build integration

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/token"
	"github.com/ssr0016/pastoral-reports/backend/internal/config"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

// A user's open sign-in stops working at once when an admin changes their status:
// the access token by tokens_valid_after, the refresh token by the revoke, and, if
// both were missed, by the status check on /auth/me.
func TestUserStatus_EndsAnOpenSession(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	ctx := context.Background()
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := repository.NewUserRepo(db)
	roles := repository.NewRoleRepo(db)

	var adminRole int64
	require.NoError(t, tdb.Pool.QueryRow(ctx, `INSERT INTO roles (name, description) VALUES ('admin', 't') RETURNING id`).Scan(&adminRole))
	admin, err := users.Create(ctx, model.NewUser{Email: "a@example.com", Name: "A", PasswordHash: "h", RoleID: adminRole})
	require.NoError(t, err)
	target, err := users.CreateWithPassword(ctx, "t@example.com", "T", "h")
	require.NoError(t, err)

	tokens := token.New(token.Config{Secret: []byte("0123456789abcdef0123456789abcdef"), Issuer: "i", Audience: "a", AccessTTL: time.Minute})
	authH := NewAuthHandler(service.NewAuthService(users, roles, nil, nil, nil, nil), NewCookies(config.AuthConfig{}, false))
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(middleware.Authenticate(tokens))
	e.GET("/test/sign-in", func(c echo.Context) error {
		access, exp, err := tokens.Issue(target.ID, "family-1")
		if err != nil {
			return err
		}
		NewCookies(config.AuthConfig{}, false).SetAccess(c, access, exp)
		return c.NoContent(http.StatusNoContent)
	})
	e.GET("/auth/me", authH.Me, middleware.RequireAuth())

	serve := func(path, cookie string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	signIn := serve("/test/sign-in", "")
	require.Equal(t, http.StatusNoContent, signIn.Code)
	cookie := signIn.Result().Cookies()[0]
	cookieHeader := cookie.Name + "=" + cookie.Value
	require.Equal(t, http.StatusOK, serve("/auth/me", cookieHeader).Code, "the session works before the change")

	svc := service.NewUserService(users, roles, repository.NewAuditRepo(db), db)
	_, err = svc.ChangeStatus(ctx, admin.ID, "", target.ID, model.UserStatusInactive)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, serve("/auth/me", cookieHeader).Code, "the revoked access token still works")

	// The net: a token signed after the revoke (as a refresh would have done) is still
	// refused, because /auth/me checks the status.
	fresh, _, err := tokens.Issue(target.ID, "family-2")
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, serve("/auth/me", middleware.AccessCookieName+"="+fresh).Code,
		"a fresh token of an inactive user still works")
}
