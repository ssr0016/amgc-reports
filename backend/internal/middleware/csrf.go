package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/ssr0016/template/internal/apperror"
)

const (
	csrfTokenKey   = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
	csrfCookieName = "csrf_token"
)

// CSRFProtection implements the double-submit cookie pattern.
func CSRFProtection() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			method := c.Request().Method

			cookie, err := c.Cookie(csrfCookieName)
			var token string
			if err == nil && cookie.Value != "" {
				token = cookie.Value
			}

			if token == "" {
				token = generateCSRFToken()
				secure := c.Scheme() == "https"
				c.SetCookie(newCSRFCookie(token, secure))
			}

			c.Set(csrfTokenKey, token)

			if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
				return next(c)
			}

			headerToken := c.Request().Header.Get(csrfHeaderName)
			if headerToken == "" || headerToken != token {
				return apperror.Forbidden("CSRF token invalid or missing")
			}

			return next(c)
		}
	}
}

// newCSRFCookie creates a CSRF cookie with secure attributes.
func newCSRFCookie(token string, secure bool) *http.Cookie {
	return &http.Cookie{ //nolint:gosec // G124: HttpOnly and SameSite set; Secure based on env
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   secure,
		MaxAge:   86400,
	}
}

// GetCSRFToken returns the current CSRF token.
func GetCSRFToken(c echo.Context) string {
	if token, ok := c.Get(csrfTokenKey).(string); ok {
		return token
	}
	return ""
}

func generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}
