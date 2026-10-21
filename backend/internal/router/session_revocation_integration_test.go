//go:build integration

package router

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol/acimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/authimpl"
	authrest "github.com/ssr0016/pastoral-reports/backend/internal/auth/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/token"
	"github.com/ssr0016/pastoral-reports/backend/internal/config"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	ourmiddleware "github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

const mePath = "/api/v1/auth/me"

// do sends one request through the real routes and token middleware.
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

// authEnv wires the real routes, auth service and token middleware over a real database.
type authEnv struct {
	h       http.Handler
	users   user.Store
	resets  auth.PasswordResetStore
	pool    *pgxpool.Pool
	tokens  *token.Manager
	refresh auth.RefreshStore
}

func newAuthEnv(t *testing.T, tokenTTL time.Duration) *authEnv {
	t.Helper()
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	env := &authEnv{
		users:   userimpl.NewStore(db),
		resets:  authimpl.NewPasswordResetStore(db),
		pool:    tdb.Pool,
		refresh: authimpl.NewRefreshStore(db),
		tokens: token.New(token.Config{
			Secret: []byte("0123456789abcdef0123456789abcdef"), Issuer: "i", Audience: "a", AccessTTL: tokenTTL,
		}),
	}
	log := slog.New(slog.DiscardHandler)
	authService := authimpl.NewService(env.users, acimpl.NewRoleStore(db), nil, auditimpl.NewMockStore(), db, log, env.tokens, env.refresh, authimpl.TokenSettings{RefreshTTL: time.Hour, RefreshIdleTTL: time.Hour})

	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	Setup(e, ourmiddleware.Authenticate(env.tokens), env.users, nil,
		authrest.NewAuthHandler(authService, authrest.NewCookies(config.AuthConfig{}, false)),
		nil, nil, nil, nil,
		authrest.NewVerificationHandler(nil),
		authrest.NewPasswordResetHandler(authimpl.NewPasswordResetService(env.users, env.resets, log)),
		nil, nil, noLimits,
	)
	env.h = e
	return env
}

func (env *authEnv) addUser(t *testing.T, email, password string) *user.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	u, err := env.users.CreateWithPassword(context.Background(), email, email, string(hash))
	require.NoError(t, err)
	return u
}

// signIn logs in and returns the two cookies the browser would hold.
func (env *authEnv) signIn(t *testing.T, email, password string) (access, refresh *http.Cookie) {
	t.Helper()
	rec := do(t, env.h, http.MethodPost, loginPath, `{"email":"`+email+`","password":"`+password+`"}`)
	require.Equal(t, http.StatusOK, rec.Code, "login: %s", rec.Body.String())
	return cookieNamed(t, rec, ourmiddleware.AccessCookieName), cookieNamed(t, rec, "refresh_token")
}

func cookieNamed(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %s cookie in %v", name, rec.Result().Cookies())
	return nil
}

// A password reset must end the sign-ins that already exist, otherwise whoever
// holds a stolen cookie keeps the account after the owner "fixes" it. Both halves
// matter: the access token (tokens_valid_after) and the refresh token (revoked).
func TestResetPassword_EndsExistingSignIn(t *testing.T) {
	env := newAuthEnv(t, time.Minute)
	const oldPassword = "old-password-123456"
	user := env.addUser(t, "victim@example.com", oldPassword)

	access, refresh := env.signIn(t, "victim@example.com", oldPassword)
	require.Equal(t, http.StatusOK, do(t, env.h, http.MethodGet, mePath, "", access).Code,
		"precondition: the access token works before the reset")

	reset, err := env.resets.Create(context.Background(), user.ID)
	require.NoError(t, err)
	rec := do(t, env.h, http.MethodPost, resetPasswordPath,
		`{"token":"`+reset.Token+`","new_password":"new-password-123456","confirm_password":"new-password-123456"}`)
	require.Equal(t, http.StatusOK, rec.Code, "reset: %s", rec.Body.String())

	assert.Equal(t, http.StatusUnauthorized, do(t, env.h, http.MethodGet, mePath, "", access).Code,
		"the old access token survived the password reset")
	assert.Equal(t, http.StatusUnauthorized, do(t, env.h, http.MethodPost, refreshPath, "", refresh).Code,
		"the old refresh token survived the password reset")
}

// Changing your own password: the device you are on stays signed in under a new
// access token, your other devices are signed out, and someone else's sign-in is
// untouched. (The audit repository is a mock: the CHANGE_PASSWORD action joins the
// database CHECK constraint in its own migration.)
func TestChangePassword_KeepsThisDeviceAndEndsTheRest(t *testing.T) {
	env := newAuthEnv(t, time.Minute)
	const oldPassword, newPassword = "old-Passw0rd-123456", "new-Passw0rd-Zq81-mX"
	env.addUser(t, "ana@example.com", oldPassword)
	env.addUser(t, "ben@example.com", oldPassword)

	here, hereRefresh := env.signIn(t, "ana@example.com", oldPassword)
	phone, phoneRefresh := env.signIn(t, "ana@example.com", oldPassword)
	bens, bensRefresh := env.signIn(t, "ben@example.com", oldPassword)

	rec := do(t, env.h, http.MethodPost, "/api/v1/auth/change-password",
		`{"current_password":"`+oldPassword+`","new_password":"`+newPassword+`","confirm_password":"`+newPassword+`"}`, here)
	require.Equal(t, http.StatusNoContent, rec.Code, "change: %s", rec.Body.String())
	renewed := cookieNamed(t, rec, ourmiddleware.AccessCookieName)

	assert.NotEqual(t, here.Value, renewed.Value, "the access token was not renewed")
	assert.Equal(t, http.StatusOK, do(t, env.h, http.MethodGet, mePath, "", renewed).Code, "this device was signed out")
	assert.Equal(t, http.StatusUnauthorized, do(t, env.h, http.MethodGet, mePath, "", here).Code, "the old access token still works")
	assert.Equal(t, http.StatusNoContent, do(t, env.h, http.MethodPost, refreshPath, "", hereRefresh).Code, "this device's refresh token was ended")
	assert.Equal(t, http.StatusUnauthorized, do(t, env.h, http.MethodGet, mePath, "", phone).Code, "another device's access token stayed valid")
	assert.Equal(t, http.StatusUnauthorized, do(t, env.h, http.MethodPost, refreshPath, "", phoneRefresh).Code, "another device stayed signed in")
	assert.Equal(t, http.StatusOK, do(t, env.h, http.MethodGet, mePath, "", bens).Code, "someone else's access token was ended")
	assert.Equal(t, http.StatusNoContent, do(t, env.h, http.MethodPost, refreshPath, "", bensRefresh).Code, "someone else's sign-in was ended")

	stored, err := env.users.GetByEmail(context.Background(), "ana@example.com")
	require.NoError(t, err)
	assert.Error(t, bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(oldPassword)), "the old password still matches")
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(newPassword)), "the new password does not match")
}

const refreshPath = "/api/v1/auth/refresh"

// Refresh swaps the pair; the old refresh token is then refused, and presenting it
// after the grace window ends the whole sign-in (theft detection).
func TestRefresh_RotatesAndReuseEndsTheSignIn(t *testing.T) {
	env := newAuthEnv(t, time.Minute)
	env.addUser(t, "ana@example.com", "pw-pw-pw-pw-123")
	_, first := env.signIn(t, "ana@example.com", "pw-pw-pw-pw-123")

	rec := do(t, env.h, http.MethodPost, refreshPath, "", first)
	require.Equal(t, http.StatusNoContent, rec.Code, "refresh: %s", rec.Body.String())
	second := cookieNamed(t, rec, "refresh_token")
	newAccess := cookieNamed(t, rec, ourmiddleware.AccessCookieName)
	assert.NotEqual(t, first.Value, second.Value, "the refresh token was not rotated")
	assert.Equal(t, http.StatusOK, do(t, env.h, http.MethodGet, mePath, "", newAccess).Code)

	// Inside the grace window the old token is only refused (a second tab), and the
	// refusal must not clear the cookies: the jar is shared and holds the new tokens.
	replay := do(t, env.h, http.MethodPost, refreshPath, "", first)
	assert.Equal(t, http.StatusUnauthorized, replay.Code)
	assert.Empty(t, replay.Result().Cookies(), "a replayed token cleared the shared cookies")
	assert.Equal(t, http.StatusNoContent, do(t, env.h, http.MethodPost, refreshPath, "", second).Code,
		"a quick replay of the old token ended the sign-in")

	// Later reuse is theft: age the rotation past the grace window.
	_, err := env.pool.Exec(context.Background(), `UPDATE refresh_tokens SET revoked_at = NOW() - interval '1 minute' WHERE replaced_by IS NOT NULL`)
	require.NoError(t, err)
	theft := do(t, env.h, http.MethodPost, refreshPath, "", first)
	assert.Equal(t, http.StatusUnauthorized, theft.Code)
	assert.Negative(t, cookieNamed(t, theft, "refresh_token").MaxAge, "a real reuse must clear the cookies")
	var live int
	require.NoError(t, env.pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM refresh_tokens WHERE revoked_at IS NULL`).Scan(&live))
	assert.Zero(t, live, "the family survived a reuse")
}

// Logout needs only the refresh cookie and ends the sign-in.
func TestLogout_EndsTheSignInAndClearsCookies(t *testing.T) {
	env := newAuthEnv(t, time.Minute)
	env.addUser(t, "ana@example.com", "pw-pw-pw-pw-123")
	_, refresh := env.signIn(t, "ana@example.com", "pw-pw-pw-pw-123")

	rec := do(t, env.h, http.MethodPost, "/api/v1/auth/logout", "", refresh) // no access cookie: it may have expired
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Negative(t, cookieNamed(t, rec, ourmiddleware.AccessCookieName).MaxAge)
	assert.Negative(t, cookieNamed(t, rec, "refresh_token").MaxAge)
	assert.Equal(t, http.StatusUnauthorized, do(t, env.h, http.MethodPost, refreshPath, "", refresh).Code)
	assert.Equal(t, http.StatusOK, do(t, env.h, http.MethodPost, "/api/v1/auth/logout", "").Code, "logging out twice, or signed out, must still answer 200")
}

// A refresh token that was never issued is a plain 401.
func TestRefresh_UnknownOrMissingIs401(t *testing.T) {
	env := newAuthEnv(t, time.Minute)
	assert.Equal(t, http.StatusUnauthorized, do(t, env.h, http.MethodPost, refreshPath, "").Code)
	assert.Equal(t, http.StatusUnauthorized, do(t, env.h, http.MethodPost, refreshPath, "",
		&http.Cookie{Name: "refresh_token", Value: "made-up"}).Code)
}

// An account that is no longer active cannot refresh, and its sign-in ends.
func TestRefresh_InactiveAccountIsRefused(t *testing.T) {
	env := newAuthEnv(t, time.Minute)
	u := env.addUser(t, "ana@example.com", "pw-pw-pw-pw-123")
	_, refresh := env.signIn(t, "ana@example.com", "pw-pw-pw-pw-123")
	_, err := env.pool.Exec(context.Background(), `UPDATE users SET status = 'inactive' WHERE id = $1`, u.ID)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, do(t, env.h, http.MethodPost, refreshPath, "", refresh).Code)
}
