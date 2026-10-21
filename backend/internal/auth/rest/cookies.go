package rest

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/config"
	"github.com/ssr0016/pastoral-reports/backend/internal/middleware"
)

const (
	refreshCookieName = "refresh_token"
	// refreshCookiePath keeps the long-lived token off every other request: the
	// browser sends it only to the auth routes (refresh and logout).
	refreshCookiePath = "/api/v1/auth"
)

// Cookies writes the two auth cookies. Both are HttpOnly, so scripts never see a
// token; SameSite=Lax and the CSRF double-submit (middleware.CSRFProtection)
// guard the writes.
type Cookies struct {
	domain string // empty: host-only, until D-F1 sets the shared domain
	secure bool
}

func NewCookies(cfg config.AuthConfig, secure bool) *Cookies {
	return &Cookies{domain: cfg.CookieDomain, secure: secure}
}

func (k *Cookies) set(c echo.Context, name, value, path string, expires time.Time) {
	maxAge := int(time.Until(expires).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	k.write(c, name, value, path, maxAge)
}

func (k *Cookies) write(c echo.Context, name, value, path string, maxAge int) {
	c.SetCookie(&http.Cookie{ //nolint:gosec // G124: HttpOnly and SameSite set; Secure follows the stage
		Name: name, Value: value, Path: path, Domain: k.domain, MaxAge: maxAge,
		HttpOnly: true, Secure: k.secure, SameSite: http.SameSiteLaxMode,
	})
}

// setPair stores a sign-in's tokens.
func (k *Cookies) setPair(c echo.Context, p *auth.TokenPair) {
	k.SetAccess(c, p.Access, p.AccessExpires)
	k.set(c, refreshCookieName, p.Refresh, refreshCookiePath, p.RefreshExpires)
}

// SetAccess stores a new access token only (the refresh token is unchanged).
func (k *Cookies) SetAccess(c echo.Context, access string, expires time.Time) {
	k.set(c, middleware.AccessCookieName, access, "/", expires)
}

// clear removes both cookies; the paths and domain must match the ones they were set with.
func (k *Cookies) clear(c echo.Context) {
	k.write(c, middleware.AccessCookieName, "", "/", -1)
	k.write(c, refreshCookieName, "", refreshCookiePath, -1)
}

func refreshCookie(c echo.Context) string {
	if ck, err := c.Cookie(refreshCookieName); err == nil {
		return ck.Value
	}
	return ""
}
