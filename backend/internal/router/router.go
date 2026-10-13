package router

import (
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"github.com/ssr0016/pastoral-reports/backend/internal/handler"
	ourmiddleware "github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

func Setup(
	e *echo.Echo,
	sm *scs.SessionManager,
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	authHandler *handler.AuthHandler,
	userHandler *handler.UserHandler,
	roleHandler *handler.RoleHandler,
	permissionHandler *handler.PermissionHandler,
	adminUserHandler *handler.AdminUserHandler,
	verificationHandler *handler.VerificationHandler,
	passwordResetHandler *handler.PasswordResetHandler,
	reportHandler *handler.ReportHandler,
	reportWriteLimit echo.MiddlewareFunc,
) {
	api := e.Group("/api/v1")

	// ============================================================
	// Public routes
	// ============================================================
	auth := api.Group("/auth")
	authLimit := ourmiddleware.RateLimit(rate.Limit(5.0/60.0), 3)
	auth.GET("/csrf", handler.CSRFToken)
	auth.POST("/register", authHandler.Register, authLimit)
	auth.POST("/login", authHandler.Login, authLimit)

	// Token flows: one bucket per route, so they cannot starve each other or
	// the login/register bucket. Each RateLimit call owns its own store.
	auth.GET("/verify-email", verificationHandler.VerifyEmail,
		ourmiddleware.RateLimit(rate.Every(time.Minute), 3))
	auth.POST("/forgot-password", passwordResetHandler.ForgotPassword,
		ourmiddleware.RateLimit(rate.Every(10*time.Minute), 3))
	auth.POST("/reset-password", passwordResetHandler.ResetPassword,
		ourmiddleware.RateLimit(rate.Every(time.Minute), 3))

	// ============================================================
	// Authenticated routes
	// ============================================================
	authProtected := auth.Group("", ourmiddleware.RequireAuth(sm))
	authProtected.POST("/logout", authHandler.Logout)
	authProtected.GET("/me", authHandler.Me)

	users := api.Group("/users", ourmiddleware.RequireAuth(sm))
	users.GET("", userHandler.ListUsers)
	users.GET("/:id", userHandler.GetUser)

	// ============================================================
	// Reports (permission-gated; the service applies the _any ownership scope)
	// ============================================================
	perm := func(name string) echo.MiddlewareFunc {
		return ourmiddleware.RequirePermission(roleRepo, userRepo, sm, name)
	}
	reports := api.Group("/reports", ourmiddleware.RequireAuth(sm))
	reports.GET("", reportHandler.ListReports, perm("reports:read"))
	// One shared bucket for all four writes (each costs a transaction and an audit
	// row). It runs after RequirePermission, so only callers who may write use
	// tokens; a user without the permission cannot exhaust a bucket others share.
	reports.POST("", reportHandler.CreateReport, perm("reports:write"), reportWriteLimit)
	reports.GET("/:id", reportHandler.GetReport, perm("reports:read"))
	reports.PUT("/:id", reportHandler.UpdateReport, perm("reports:write"), reportWriteLimit)
	reports.DELETE("/:id", reportHandler.SoftDeleteReport, perm("reports:delete"), reportWriteLimit)
	reports.PATCH("/:id/complete", reportHandler.ToggleComplete, perm("reports:approve"), reportWriteLimit)

	// ============================================================
	// Admin routes (RBAC-protected)
	// ============================================================
	admin := api.Group("/admin",
		ourmiddleware.RequireAuth(sm),
		ourmiddleware.RequireRole(userRepo, sm, "admin"),
	)

	// Roles
	admin.GET("/roles", roleHandler.List)
	admin.POST("/roles", roleHandler.Create)
	admin.GET("/roles/:id", roleHandler.Get)
	admin.PUT("/roles/:id", roleHandler.Update)
	admin.DELETE("/roles/:id", roleHandler.Delete)
	admin.POST("/roles/:id/permissions", roleHandler.AssignPermission)
	admin.DELETE("/roles/:id/permissions/:pid", roleHandler.RevokePermission)

	// Permissions
	admin.GET("/permissions", permissionHandler.List)
	admin.POST("/permissions", permissionHandler.Create)
	admin.GET("/permissions/:id", permissionHandler.Get)
	admin.DELETE("/permissions/:id", permissionHandler.Delete)

	// Users (admin management)
	admin.GET("/users/:id", adminUserHandler.GetUser)
	admin.PUT("/users/:id/role", adminUserHandler.ChangeRole)

	_ = roleRepo // referenced in RequireRole
}
