package middleware

import (
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/token"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
)

// AccessCookieName is the HttpOnly cookie that carries the access token.
const AccessCookieName = "access_token"

// Authenticate puts the user an access token proves into the request context.
// The token is read from the access cookie, or, with no cookie, from an
// "Authorization: Bearer" header (for tooling). A missing, invalid or expired
// token leaves the context without a user; it never refuses a request itself,
// RequireAuth does that per group. So an unauthenticated caller meets the same
// 401 whatever was wrong with the token.
func Authenticate(tokens *token.Manager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if raw := accessToken(c); raw != "" {
				if id, err := tokens.Parse(raw); err == nil {
					req := c.Request()
					u := request.User{ID: id.UserID, SessionID: id.SessionID, IssuedAt: id.IssuedAt}
					c.SetRequest(req.WithContext(request.WithUser(req.Context(), u)))
				}
			}
			return next(c)
		}
	}
}

func accessToken(c echo.Context) string {
	if cookie, err := c.Cookie(AccessCookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	if h := c.Request().Header.Get(echo.HeaderAuthorization); len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// RequireAuth answers 401 unless Authenticate found a user.
func RequireAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if _, ok := request.UserFrom(c.Request().Context()); !ok {
				return apperror.Unauthorized("authentication required")
			}
			return next(c)
		}
	}
}
