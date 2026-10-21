package middleware

import (
	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

// RequireRole returns a middleware that requires the user to have one of the specified roles.
func RequireRole(userRepo user.Store, roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			caller, ok := request.UserFrom(c.Request().Context())
			if !ok {
				return apperror.Unauthorized("authentication required")
			}

			usr, err := userRepo.GetWithRole(c.Request().Context(), caller.ID)
			if err != nil {
				return apperror.Internal("failed to load user").WithError(err)
			}
			// A token that outlives the account's status (D-A10), or that was signed before a
			// password change or reset (tokens_valid_after), stops working here.
			if usr == nil || !usr.CanSignIn() || usr.AccessRevoked(caller.IssuedAt) {
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
func RequirePermission(roleRepo accesscontrol.RoleStore, userRepo user.Store, perms ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			caller, ok := request.UserFrom(c.Request().Context())
			if !ok {
				return apperror.Unauthorized("authentication required")
			}

			usr, err := userRepo.GetWithRole(c.Request().Context(), caller.ID)
			if err != nil {
				return apperror.Internal("failed to load user").WithError(err)
			}
			// A token that outlives the account's status (D-A10), or that was signed before a
			// password change or reset (tokens_valid_after), stops working here.
			if usr == nil || !usr.CanSignIn() || usr.AccessRevoked(caller.IssuedAt) {
				return apperror.Unauthorized("authentication required")
			}

			if usr.RoleID == 0 {
				return apperror.Forbidden("user has no role assigned")
			}

			// Load permissions for user's role
			userPerms, err := roleRepo.GetPermissions(c.Request().Context(), usr.RoleID)
			if err != nil {
				return apperror.Internal("failed to load permissions").WithError(err)
			}

			// Build permission set
			permSet := make(map[string]bool, len(userPerms))
			for _, p := range userPerms {
				permSet[p.Name] = true
			}

			// Check all required permissions
			for _, required := range perms {
				if !permSet[required] {
					return apperror.Forbidden("insufficient permissions: " + required)
				}
			}

			return next(c)
		}
	}
}
