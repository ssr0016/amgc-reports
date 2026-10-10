package middleware

import (
	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"
	"github.com/ssr0016/template/internal/apperror"
)

const UserIDKey = "user_id"

func RequireAuth(sm *scs.SessionManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID := sm.GetInt64(c.Request().Context(), UserIDKey)
			if userID == 0 {
				return apperror.Unauthorized("authentication required")
			}
			return next(c)
		}
	}
}
