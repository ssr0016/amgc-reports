package middleware

import (
	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

// LoadCaller reads the signed-in user, their role and its permissions in one query
// and puts them in the request context (user.WithCaller). RequireRole,
// RequirePermission and the services read them from there, so a request loads its
// caller once. It runs after RequireAuth, in every group whose routes are gated.
func LoadCaller(users user.Store) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			caller, ok := request.UserFrom(ctx)
			if !ok {
				return apperror.Unauthorized("authentication required")
			}

			usr, err := users.GetCaller(ctx, caller.ID)
			if err != nil {
				return apperror.Internal("failed to load user").WithError(err)
			}
			// A token that outlives the account's status (D-A10), or that was signed before a
			// password change or reset (tokens_valid_after), stops working here.
			if usr == nil || !usr.CanSignIn() || usr.AccessRevoked(caller.IssuedAt) {
				return apperror.Unauthorized("authentication required")
			}

			c.SetRequest(c.Request().WithContext(user.WithCaller(ctx, usr)))
			return next(c)
		}
	}
}

// loadedCaller returns the caller LoadCaller stored for this request.
func loadedCaller(c echo.Context) (*user.Caller, bool) {
	ctx := c.Request().Context()
	caller, ok := request.UserFrom(ctx)
	if !ok {
		return nil, false
	}
	return user.CallerFrom(ctx, caller.ID)
}

// RequireRole returns a middleware that requires the user to have one of the specified roles.
// It runs after LoadCaller.
func RequireRole(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			usr, ok := loadedCaller(c)
			if !ok {
				return apperror.Unauthorized("authentication required")
			}

			if usr.Role == nil {
				return apperror.Forbidden("user has no role assigned")
			}

			for _, role := range roles {
				if usr.Role.Name == role {
					return next(c)
				}
			}

			return apperror.Forbidden("insufficient role permissions")
		}
	}
}

// RequirePermission returns a middleware that requires the user to have all specified permissions.
// It runs after LoadCaller.
func RequirePermission(perms ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			usr, ok := loadedCaller(c)
			if !ok {
				return apperror.Unauthorized("authentication required")
			}

			if usr.RoleID == 0 {
				return apperror.Forbidden("user has no role assigned")
			}

			for _, required := range perms {
				if !usr.Permissions[required] {
					return apperror.Forbidden("insufficient permissions: " + required)
				}
			}

			return next(c)
		}
	}
}
