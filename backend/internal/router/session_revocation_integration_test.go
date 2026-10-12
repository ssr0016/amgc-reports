//go:build integration

package router

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/handler"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
	"github.com/ssr0016/pastoral-reports/backend/internal/session"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

const mePath = "/api/v1/auth/me"

// do sends one request through the session middleware and the real routes.
func do(t *testing.T, h http.Handler, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// A password reset must end the sessions that already exist, otherwise whoever
// holds a stolen cookie keeps the account after the owner "fixes" it.
func TestResetPassword_EndsExistingSession(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	ctx := context.Background()

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := repository.NewUserRepo(db)
	resets := repository.NewPasswordResetRepo(db)
	log := slog.New(slog.DiscardHandler)
	sm := session.New(tdb.Pool)

	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	Setup(e, sm, users, nil,
		handler.NewAuthHandler(service.NewAuthService(users, nil, log), sm),
		nil, nil, nil, nil,
		handler.NewVerificationHandler(nil),
		handler.NewPasswordResetHandler(service.NewPasswordResetService(users, resets, log)),
	)
	h := sm.LoadAndSave(e)

	const oldPassword = "old-password-123456"
	hash, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.MinCost)
	require.NoError(t, err)
	user, err := users.CreateWithPassword(ctx, "victim@example.com", "Victim", string(hash))
	require.NoError(t, err)

	login := do(t, h, http.MethodPost, loginPath,
		`{"email":"victim@example.com","password":"`+oldPassword+`"}`)
	require.Equal(t, http.StatusOK, login.Code, "login: %s", login.Body.String())
	cookies := login.Result().Cookies()
	require.NotEmpty(t, cookies, "login set no session cookie")
	require.Equal(t, http.StatusOK, do(t, h, http.MethodGet, mePath, "", cookies...).Code,
		"precondition: the session works before the reset")

	token, err := resets.Create(ctx, user.ID)
	require.NoError(t, err)
	reset := do(t, h, http.MethodPost, resetPasswordPath,
		`{"token":"`+token.Token+`","new_password":"new-password-123456"}`)
	require.Equal(t, http.StatusOK, reset.Code, "reset: %s", reset.Body.String())

	after := do(t, h, http.MethodGet, mePath, "", cookies...)
	require.Equal(t, http.StatusUnauthorized, after.Code, "the old session survived the password reset")
}
