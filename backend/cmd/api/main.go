package main

// @title           Template Go Echo API
// @version         1.0
// @description     Go Echo + Squirrel + pgx template with session auth
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@example.com

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name app_session

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	echoSwagger "github.com/swaggo/echo-swagger"

	_ "github.com/ssr0016/pastoral-reports/backend/docs"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/config"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/handler"
	"github.com/ssr0016/pastoral-reports/backend/internal/logger"
	ourmiddleware "github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/router"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
	"github.com/ssr0016/pastoral-reports/backend/internal/session"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

const usage = `usage: api [migrate]

  (no arguments)  start the HTTP server
  migrate         apply database migrations and exit
`

// commands are what dispatch can run. They are fields so tests can replace
// them without a database or a listening server.
type commands struct {
	run     func() error
	migrate func() error
}

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stderr, commands{run: run, migrate: runMigrate}))
}

// dispatch runs the subcommand named by args and returns the process exit
// code: 0 on success, 1 when the command fails, 2 for a usage error. An
// unrecognised argument is an error rather than "start the server", so a typo
// in a pre-deploy command cannot silently launch the server instead.
func dispatch(args []string, stderr io.Writer, cmds commands) int {
	var cmd func() error
	switch {
	case len(args) == 0:
		cmd = cmds.run
	case args[0] == "migrate" && len(args) == 1:
		cmd = cmds.migrate
	case args[0] == "migrate":
		_, _ = fmt.Fprintf(stderr, "unexpected argument %q for migrate\n%s", args[1], usage)
		return 2
	default:
		_, _ = fmt.Fprintf(stderr, "unknown subcommand: %q\n%s", args[0], usage)
		return 2
	}

	if err := cmd(); err != nil {
		slog.Error("fatal error", "error", err)
		return 1
	}
	return 0
}

func run() error {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logger.New(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	})
	slog.SetDefault(log)

	log.Info("starting server",
		"env", cfg.App.Env,
		"stage", cfg.Stage.String(),
		"log_level", cfg.Logging.Level,
		"log_format", cfg.Logging.Format,
	)

	if cfg.Proxy.UsedDefault {
		log.Warn("TRUSTED_PROXY_CIDRS not set, using Railway default " + config.DefaultTrustedProxyCIDR +
			". Set this if deploying elsewhere.")
	}

	ctx := context.Background()
	db, err := database.NewWithURL(ctx, cfg.Database.URL)
	if err != nil {
		return err
	}
	defer db.Pool.Close()

	if !cfg.IsProduction() {
		if err := database.RunMigrations(db.Pool); err != nil {
			return err
		}
	}

	if err := database.SeedData(ctx, db, log, database.AdminSeed{
		Email:    cfg.Admin.Email,
		Name:     cfg.Admin.Name,
		Password: cfg.Admin.Password,
	}); err != nil {
		return err
	}

	sm := session.New(db.Pool)

	// Repositories
	userRepo := repository.NewUserRepo(db)
	roleRepo := repository.NewRoleRepo(db)
	permissionRepo := repository.NewPermissionRepo(db)
	verificationRepo := repository.NewVerificationRepo(db)
	passwordResetRepo := repository.NewPasswordResetRepo(db)

	// Services
	verificationService := service.NewVerificationService(userRepo, verificationRepo, log)
	authService := service.NewAuthService(userRepo, verificationService)
	passwordResetService := service.NewPasswordResetService(userRepo, passwordResetRepo, log)

	// Handlers
	authHandler := handler.NewAuthHandler(authService, sm)
	userHandler := handler.NewUserHandler(userRepo)
	roleHandler := handler.NewRoleHandler(roleRepo, permissionRepo)
	permissionHandler := handler.NewPermissionHandler(permissionRepo)
	adminUserHandler := handler.NewAdminUserHandler(userRepo, roleRepo)
	verificationHandler := handler.NewVerificationHandler(verificationService)
	passwordResetHandler := handler.NewPasswordResetHandler(passwordResetService)
	healthHandler := handler.NewHealthHandler(db.Pool)

	e := echo.New()
	e.IPExtractor = ourmiddleware.TrustedProxyExtractor(cfg.Proxy.TrustedCIDRs)
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.HideBanner = true
	e.HidePort = true

	// Global middleware
	e.Use(middleware.RequestID())
	e.Use(ourmiddleware.WarnIfNoTrustedProxy(log, cfg.Proxy.TrustedCIDRs, ourmiddleware.TrustedProxyWarnWindow))
	e.Use(ourmiddleware.SlogLogger(log))
	e.Use(ourmiddleware.PrometheusMiddleware())
	e.Use(ourmiddleware.CSRFProtection())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     cfg.CORS.AllowedOrigins,
		AllowCredentials: true,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, echo.HeaderXCSRFToken},
	}))

	// Swagger
	e.GET("/swagger/*", echoSwagger.WrapHandler)

	// Observability endpoints
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	e.GET("/health", healthHandler.HealthCheck)
	e.GET("/ready", healthHandler.Readiness)
	e.GET("/live", healthHandler.Liveness)

	// API routes
	router.Setup(
		e,
		sm,
		userRepo,
		roleRepo,
		authHandler,
		userHandler,
		roleHandler,
		permissionHandler,
		adminUserHandler,
		verificationHandler,
		passwordResetHandler,
	)

	scsHandler := sm.LoadAndSave(e)

	server := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      scsHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("server starting",
			"port", cfg.App.Port,
			"api_url", cfg.App.URL,
			"swagger_url", cfg.App.URL+"/swagger/index.html",
			"health_url", cfg.App.URL+"/health",
			"ready_url", cfg.App.URL+"/ready",
			"live_url", cfg.App.URL+"/live",
			"metrics_url", cfg.App.URL+"/metrics",
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			quit <- syscall.SIGTERM
		}
	}()

	<-quit

	log.Info("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "error", err)
		return err
	}

	log.Info("server stopped")
	return nil
}
