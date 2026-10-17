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
	"github.com/stretchr/testify/assert"
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
		handler.NewAuthHandler(service.NewAuthService(users, repository.NewRoleRepo(db), nil, nil, nil, log), sm),
		nil, nil, nil, nil,
		handler.NewVerificationHandler(nil),
		handler.NewPasswordResetHandler(service.NewPasswordResetService(users, resets, log)),
		nil, nil, noLimits,
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
		`{"token":"`+token.Token+`","new_password":"new-password-123456","confirm_password":"new-password-123456"}`)
	require.Equal(t, http.StatusOK, reset.Code, "reset: %s", reset.Body.String())

	after := do(t, h, http.MethodGet, mePath, "", cookies...)
	require.Equal(t, http.StatusUnauthorized, after.Code, "the old session survived the password reset")
}

// Changing your own password through the real session store: the device you are on
// stays signed in under a new token, your other devices are signed out, and
// someone else's session is untouched. (The audit repository is a mock: the
// CHANGE_PASSWORD action joins the database CHECK constraint in its own migration.)
func TestChangePassword_KeepsThisDeviceAndEndsTheRest(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	ctx := context.Background()

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := repository.NewUserRepo(db)
	log := slog.New(slog.DiscardHandler)
	sm := session.New(tdb.Pool)

	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	authService := service.NewAuthService(users, repository.NewRoleRepo(db), nil,
		repository.NewMockAuditRepo(), repository.NewTxManager(tdb.Pool), log)
	Setup(e, sm, users, nil, handler.NewAuthHandler(authService, sm),
		nil, nil, nil, nil, handler.NewVerificationHandler(nil),
		handler.NewPasswordResetHandler(nil), nil, nil, noLimits,
	)
	h := sm.LoadAndSave(e)

	const oldPassword, newPassword = "old-Passw0rd-123456", "new-Passw0rd-Zq81-mX"
	hash, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.MinCost)
	require.NoError(t, err)
	_, err = users.CreateWithPassword(ctx, "ana@example.com", "Ana", string(hash))
	require.NoError(t, err)
	_, err = users.CreateWithPassword(ctx, "ben@example.com", "Ben", string(hash))
	require.NoError(t, err)

	signIn := func(email string) *http.Cookie {
		rec := do(t, h, http.MethodPost, loginPath, `{"email":"`+email+`","password":"`+oldPassword+`"}`)
		require.Equal(t, http.StatusOK, rec.Code, "login: %s", rec.Body.String())
		require.NotEmpty(t, rec.Result().Cookies())
		return rec.Result().Cookies()[0]
	}
	here, phone, bens := signIn("ana@example.com"), signIn("ana@example.com"), signIn("ben@example.com")

	rec := do(t, h, http.MethodPost, "/api/v1/auth/change-password",
		`{"current_password":"`+oldPassword+`","new_password":"`+newPassword+`","confirm_password":"`+newPassword+`"}`, here)
	require.Equal(t, http.StatusNoContent, rec.Code, "change: %s", rec.Body.String())
	require.NotEmpty(t, rec.Result().Cookies(), "no renewed session cookie")
	renewed := rec.Result().Cookies()[0]

	assert.NotEqual(t, here.Value, renewed.Value, "the token was not rotated")
	assert.Equal(t, http.StatusOK, do(t, h, http.MethodGet, mePath, "", renewed).Code, "this device was signed out")
	assert.Equal(t, http.StatusUnauthorized, do(t, h, http.MethodGet, mePath, "", here).Code, "the old token still works")
	assert.Equal(t, http.StatusUnauthorized, do(t, h, http.MethodGet, mePath, "", phone).Code, "another device stayed signed in")
	assert.Equal(t, http.StatusOK, do(t, h, http.MethodGet, mePath, "", bens).Code, "someone else's session was ended")

	// Read the stored hash: three sign-ins already used the login route's burst of 3 per address.
	stored, err := users.GetByEmail(ctx, "ana@example.com")
	require.NoError(t, err)
	assert.Error(t, bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(oldPassword)), "the old password still matches")
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(newPassword)), "the new password does not match")
}
