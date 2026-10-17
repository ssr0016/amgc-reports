//go:build integration

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
	"github.com/ssr0016/pastoral-reports/backend/internal/session"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

// A user's open session stops working at once when an admin changes their status,
// through the real session store: by the revoke, and, if that were missed, by the
// status check on /auth/me.
func TestUserStatus_EndsAnOpenSession(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	ctx := context.Background()
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := repository.NewUserRepo(db)
	roles := repository.NewRoleRepo(db)

	var adminRole int64
	require.NoError(t, tdb.Pool.QueryRow(ctx, `INSERT INTO roles (name, description) VALUES ('admin', 't') RETURNING id`).Scan(&adminRole))
	admin, err := users.Create(ctx, tdb.Pool, model.NewUser{Email: "a@example.com", Name: "A", PasswordHash: "h", RoleID: adminRole})
	require.NoError(t, err)
	target, err := users.CreateWithPassword(ctx, "t@example.com", "T", "h")
	require.NoError(t, err)

	sm := session.New(tdb.Pool)
	authH := NewAuthHandler(service.NewAuthService(users, roles, nil, nil, nil, nil), sm)
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(echo.WrapMiddleware(sm.LoadAndSave))
	e.GET("/test/sign-in", func(c echo.Context) error {
		sm.Put(c.Request().Context(), "user_id", target.ID)
		return c.NoContent(http.StatusNoContent)
	})
	e.GET("/auth/me", authH.Me, middleware.RequireAuth(sm))

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

	svc := service.NewUserService(users, roles, repository.NewAuditRepo(db), repository.NewTxManager(tdb.Pool))
	_, err = svc.ChangeStatus(ctx, admin.ID, "", target.ID, model.UserStatusInactive)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, serve("/auth/me", cookieHeader).Code, "the revoked session still works")

	// The net: even a session the revoke missed is refused, because /auth/me checks the status.
	_, err = tdb.Pool.Exec(ctx, `UPDATE sessions SET revoked_at = NULL WHERE user_id = $1`, target.ID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, serve("/auth/me", cookieHeader).Code, "an un-revoked session of an inactive user still works")
}
