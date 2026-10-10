package middleware

import (
	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"

	"github.com/ssr0016/template/internal/apperror"
	"github.com/ssr0016/template/internal/repository"
)

// RequireRole returns a middleware that requires the user to have one of the specified roles.
func RequireRole(userRepo repository.UserRepository, sm *scs.SessionManager, roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID := sm.GetInt64(c.Request().Context(), "user_id")
			if userID == 0 {
				return apperror.Unauthorized("authentication required")
			}

			user, err := userRepo.GetWithRole(c.Request().Context(), userID)
			if err != nil {
				return apperror.Internal("failed to load user").WithError(err)
			}
			if user == nil {
				return apperror.Unauthorized("user not found")
			}

			if user.Role == nil {
				return apperror.Forbidden("user has no role assigned")
			}

			for _, role := range roles {
				if user.Role.Name == role {
					return next(c)
				}
			}

			return apperror.Forbidden("insufficient role permissions")
		}
	}
}

// RequirePermission returns a middleware that requires the user to have all specified permissions.
func RequirePermission(roleRepo repository.RoleRepository, userRepo repository.UserRepository, sm *scs.SessionManager, perms ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID := sm.GetInt64(c.Request().Context(), "user_id")
			if userID == 0 {
				return apperror.Unauthorized("authentication required")
			}

			user, err := userRepo.GetWithRole(c.Request().Context(), userID)
			if err != nil {
				return apperror.Internal("failed to load user").WithError(err)
			}
			if user == nil {
				return apperror.Unauthorized("user not found")
			}

			if user.RoleID == 0 {
				return apperror.Forbidden("user has no role assigned")
			}

			// Load permissions for user's role
			userPerms, err := roleRepo.GetPermissions(c.Request().Context(), user.RoleID)
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
