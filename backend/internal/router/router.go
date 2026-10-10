package router

import (
	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"github.com/ssr0016/template/internal/handler"
	ourmiddleware "github.com/ssr0016/template/internal/middleware"
	"github.com/ssr0016/template/internal/repository"
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
) {
	api := e.Group("/api/v1")

	// ============================================================
	// Public routes
	// ============================================================
	auth := api.Group("/auth")
	authLimit := ourmiddleware.RateLimit(rate.Limit(5.0/60.0), 3)
	auth.POST("/register", authHandler.Register, authLimit)
	auth.POST("/login", authHandler.Login, authLimit)

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
