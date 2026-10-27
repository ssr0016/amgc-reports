package router

import (
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	acrest "github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol/rest"
	auditrest "github.com/ssr0016/pastoral-reports/backend/internal/audit/rest"
	authrest "github.com/ssr0016/pastoral-reports/backend/internal/auth/rest"
	ourmiddleware "github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	reportrest "github.com/ssr0016/pastoral-reports/backend/internal/report/rest"
	statsrest "github.com/ssr0016/pastoral-reports/backend/internal/stats/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	userrest "github.com/ssr0016/pastoral-reports/backend/internal/user/rest"
)

// Limiters are the rate limiters Setup mounts. Global covers every /api/v1
// route (system routes such as /health are registered outside it and stay
// open). Read and Write guard the reports routes and run after the permission
// check, so a caller without the permission cannot use up tokens.
type Limiters struct {
	Global, Read, Write echo.MiddlewareFunc
}

func Setup(
	e *echo.Echo,
	authenticate echo.MiddlewareFunc,
	userRepo user.Store,
	authHandler *authrest.AuthHandler,
	userHandler *userrest.UserHandler,
	avatarHandler *userrest.AvatarHandler,
	roleHandler *acrest.RoleHandler,
	permissionHandler *acrest.PermissionHandler,
	adminUserHandler *userrest.AdminUserHandler,
	verificationHandler *authrest.VerificationHandler,
	passwordResetHandler *authrest.PasswordResetHandler,
	reportHandler *reportrest.ReportHandler,
	statsHandler *statsrest.StatsHandler,
	auditHandler *auditrest.AuditHandler,
	limits Limiters,
) {
	// authenticate only runs in the groups that need a user, so public routes never read a token.
	authn := authenticate

	api := e.Group("/api/v1", limits.Global)

	// ============================================================
	// Public routes
	// ============================================================
	auth := api.Group("/auth")
	authLimit := ourmiddleware.RateLimit(rate.Limit(5.0/60.0), 3)
	auth.GET("/csrf", authrest.CSRFToken)
	auth.POST("/login", authHandler.Login, authLimit)
	// Refresh and logout read only the refresh cookie, so they stay open: they must
	// work after the access token has expired. CSRF protection still applies.
	auth.POST("/refresh", authHandler.Refresh, ourmiddleware.RateLimit(rate.Every(time.Second), 10))
	auth.POST("/logout", authHandler.Logout)

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
	authProtected := auth.Group("", authn, ourmiddleware.RequireAuth())
	authProtected.GET("/me", authHandler.Me)
	// Shares the per-user write bucket. It runs after RequireAuth, so only a signed-in
	// caller uses tokens. The lockout counter, not this limit, stops password guessing.
	authProtected.POST("/change-password", authHandler.ChangePassword, limits.Write)

	// loadCaller reads the caller, role and permissions once, in front of the gates:
	// perm checks the role's permissions, adminOnly the admin role.
	loadCaller := ourmiddleware.LoadCaller(userRepo)
	perm := func(name string) echo.MiddlewareFunc { return ourmiddleware.RequirePermission(name) }
	adminOnly := ourmiddleware.RequireRole("admin")

	// Lists every account's email and name, so it needs users:list (37b), never
	// users:read: the plain user role holds users:read.
	users := api.Group("/users", authn, ourmiddleware.RequireAuth(), loadCaller)
	users.GET("", userHandler.ListUsers, perm("users:list"))
	users.GET("/:id", userHandler.GetUser, perm("users:list"))
	// Any signed-in user may set their own picture; it shares the per-user write bucket.
	users.POST("/me/avatar", avatarHandler.UploadAvatar, limits.Write)

	// ============================================================
	// Reports (permission-gated; the service applies the _any ownership scope)
	// ============================================================
	reports := api.Group("/reports", authn, ourmiddleware.RequireAuth(), loadCaller)
	reports.GET("", reportHandler.ListReports, perm("reports:read"), limits.Read)
	reports.GET("/current", reportHandler.CurrentReport, perm("reports:read"), limits.Read)
	// One shared bucket for all four writes (each costs a transaction and an audit
	// row). It runs after RequirePermission, so only callers who may write use
	// tokens; a user without the permission cannot exhaust a bucket others share.
	reports.POST("", reportHandler.CreateReport, ourmiddleware.RequireRole("user"), perm("reports:write"), limits.Write)
	reports.GET("/:id", reportHandler.GetReport, perm("reports:read"), limits.Read)
	reports.PUT("/:id", reportHandler.UpdateReport, perm("reports:write"), limits.Write)
	reports.PATCH("/:id/submit", reportHandler.SubmitReport, perm("reports:write"), limits.Write)
	reports.DELETE("/:id", reportHandler.SoftDeleteReport, perm("reports:delete"), limits.Write)
	reports.PATCH("/:id/complete", reportHandler.ToggleComplete, perm("reports:approve"), limits.Write)
	reports.PATCH("/:id/revision", reportHandler.RequestRevision, perm("reports:approve"), limits.Write)

	// ============================================================
	// Admin routes (37b): the review areas are permission-gated, so a custom
	// role can use them; account and role management stay admin-only, because
	// users:write includes resetting an admin's password.
	// ============================================================
	admin := api.Group("/admin", authn, ourmiddleware.RequireAuth(), loadCaller)

	// Roles (D-A21). Writes need roles:write on top of the admin role, and are audited;
	// PUT replaces the whole permission set, so there are no per-permission routes.
	admin.GET("/roles", roleHandler.List, adminOnly)
	admin.POST("/roles", roleHandler.Create, adminOnly, perm("roles:write"), limits.Write)
	admin.GET("/roles/:id", roleHandler.Get, adminOnly)
	admin.PUT("/roles/:id", roleHandler.Update, adminOnly, perm("roles:write"), limits.Write)
	admin.DELETE("/roles/:id", roleHandler.Delete, adminOnly, perm("roles:write"), limits.Write)

	// Permissions
	admin.GET("/permissions", permissionHandler.List, adminOnly)
	admin.POST("/permissions", permissionHandler.Create, adminOnly)
	admin.GET("/permissions/:id", permissionHandler.Get, adminOnly)
	admin.DELETE("/permissions/:id", permissionHandler.Delete, adminOnly)

	// Users (admin management)
	admin.GET("/users/:id", adminUserHandler.GetUser, adminOnly)
	// The only way to create an account (there is no self-registration).
	// users:write is a second, explicit gate after the admin role.
	admin.POST("/users", adminUserHandler.CreateUser, adminOnly, perm("users:write"))
	admin.PUT("/users/:id", adminUserHandler.UpdateUser, adminOnly, perm("users:write"))
	admin.PUT("/users/:id/status", adminUserHandler.ChangeStatus, adminOnly, perm("users:write"))
	admin.PUT("/users/:id/role", adminUserHandler.ChangeRole, adminOnly, perm("users:write"), limits.Write)
	admin.DELETE("/users/:id", adminUserHandler.DeleteUser, adminOnly, perm("users:write"))
	admin.POST("/users/:id/restore", adminUserHandler.RestoreUser, adminOnly, perm("users:write"))
	// Sets another user's password (D-A12): a write, so it shares change-password's limiter.
	admin.POST("/users/:id/reset-password", adminUserHandler.ResetPassword, adminOnly, perm("users:write"), limits.Write)

	// Excel downloads (D-A6). POST so the CSRF check covers the audit row they write.
	admin.POST("/reports/excel", reportHandler.ExportReports, perm("reports:export"), limits.Write)
	admin.POST("/reports/:id/excel", reportHandler.ExportReport, perm("reports:export"), limits.Write)
	// Preview of either download (D-A15): no side effects, so the read limiter.
	admin.POST("/reports/excel/preview", reportHandler.PreviewExcel, perm("reports:export"), limits.Read)

	// Counts for the admin panel cards, and who has reported this month.
	admin.GET("/stats", statsHandler.GetStats, perm("tracker:read"), limits.Read)
	admin.GET("/tracker", statsHandler.GetTracker, perm("tracker:read"), limits.Read)
	admin.GET("/analytics/attendance", statsHandler.GetAttendanceAnalytics, perm("tracker:read"), limits.Read)

	// The audit trail, read-only (no other method is routed).
	admin.GET("/audit-logs", auditHandler.List, perm("audit:read"), limits.Read)
}
