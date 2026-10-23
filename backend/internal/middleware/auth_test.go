package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/token"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

func testTokens(ttl time.Duration) *token.Manager {
	return token.New(token.Config{Secret: []byte("0123456789abcdef0123456789abcdef"), Issuer: "i", Audience: "a", AccessTTL: ttl})
}

// serveAuth runs one GET through Authenticate and RequireAuth and returns the status
// and the user the handler saw.
func serveAuth(m *token.Manager, mutate func(*http.Request)) (int, request.User) {
	var seen request.User
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.GET("/", func(c echo.Context) error {
		seen, _ = request.UserFrom(c.Request().Context())
		return c.NoContent(http.StatusNoContent)
	}, Authenticate(m), RequireAuth())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	mutate(req)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Code, seen
}

func TestAuthenticate(t *testing.T) {
	m := testTokens(time.Minute)
	good, _, err := m.Issue(7, "fam")
	require.NoError(t, err)
	expired, _, err := testTokens(-time.Hour).Issue(7, "fam")
	require.NoError(t, err)
	foreign, _, err := token.New(token.Config{Secret: []byte("another-secret-another-secret-12"), Issuer: "i", Audience: "a", AccessTTL: time.Minute}).Issue(7, "fam")
	require.NoError(t, err)

	cookie := func(v string) func(*http.Request) {
		return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: AccessCookieName, Value: v}) }
	}
	tests := []struct {
		name   string
		mutate func(*http.Request)
		want   int
	}{
		{"cookie", cookie(good), http.StatusNoContent},
		{"bearer header", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+good) }, http.StatusNoContent},
		{"cookie wins over a bad header", func(r *http.Request) {
			cookie(good)(r)
			r.Header.Set("Authorization", "Bearer nonsense")
		}, http.StatusNoContent},
		{"no token", func(*http.Request) {}, http.StatusUnauthorized},
		{"garbage cookie", cookie("nonsense"), http.StatusUnauthorized},
		{"expired", cookie(expired), http.StatusUnauthorized},
		{"signed with another secret", cookie(foreign), http.StatusUnauthorized},
		{"basic auth header", func(r *http.Request) { r.Header.Set("Authorization", "Basic "+good) }, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, user := serveAuth(m, tt.mutate)
			require.Equal(t, tt.want, got)
			if tt.want == http.StatusNoContent {
				require.Equal(t, int64(7), user.ID)
				require.Equal(t, "fam", user.SessionID)
				require.False(t, user.IssuedAt.IsZero())
			}
		})
	}
}

// A token signed before users.tokens_valid_after is refused by LoadCaller, in front
// of either gate (RequireRole, RequirePermission), whatever else is right with it.
func TestRBAC_RefusesATokenSignedBeforeTokensValidAfter(t *testing.T) {
	gates := map[string]echo.MiddlewareFunc{
		"RequireRole":       RequireRole("admin"),
		"RequirePermission": RequirePermission("reports:read"),
	}
	issued := time.Now().Add(-time.Minute)
	tests := []struct {
		name       string
		validAfter time.Time
		want       int
	}{
		{"never revoked", time.Time{}, http.StatusOK},
		{"revoked before the token", issued.Add(-time.Second), http.StatusOK},
		{"revoked after the token", issued.Add(time.Second), http.StatusUnauthorized},
	}
	for name, gate := range gates {
		for _, tt := range tests {
			t.Run(name+"/"+tt.name, func(t *testing.T) {
				users := userimpl.NewMockStore()
				users.Roles = allowAll{}
				users.GetWithRoleFunc = func(context.Context, int64) (*user.User, error) {
					return &user.User{ID: 1, RoleID: 9, Role: &accesscontrol.Role{Name: "admin"}, TokensValidAfter: tt.validAfter}, nil
				}
				e := echo.New()
				e.HTTPErrorHandler = apperror.ErrorHandler
				e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
					return func(c echo.Context) error {
						ctx := request.WithUser(c.Request().Context(), request.User{ID: 1, IssuedAt: issued})
						c.SetRequest(c.Request().WithContext(ctx))
						return next(c)
					}
				})
				e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, LoadCaller(users), gate)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil))
				require.Equal(t, tt.want, rec.Code)
			})
		}
	}
}
