package main

// @title           Pastoral Reports API
// @version         1.0
// @description     Pastoral Reports backend API: session auth and RBAC

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @BasePath  /api/v1

// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name app_session

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
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
	"golang.org/x/time/rate"

	_ "github.com/ssr0016/pastoral-reports/backend/docs"
	"github.com/ssr0016/pastoral-reports/backend/internal/config"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/handler"
	"github.com/ssr0016/pastoral-reports/backend/internal/logger"
	ourmiddleware "github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/router"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
	"github.com/ssr0016/pastoral-reports/backend/internal/session"
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

// registerSystemRoutes registers the Swagger UI and the observability and
// health endpoints. It is separate from run() so it can be tested without a
// database.
//
//   - /swagger/* only when SwaggerEnabled.
//   - /metrics behind Basic Auth when credentials are set (any stage); open
//     outside production without them; not served in production without
//     them. A half-set pair is never served: Validate rejects it at startup,
//     and this keeps a hand-built Config from failing open.
//   - /health, /ready and /live are always public.
func registerSystemRoutes(e *echo.Echo, cfg *config.Config, health *handler.HealthHandler) {
	obs := cfg.Observability

	if obs.SwaggerEnabled {
		e.GET("/swagger/*", echoSwagger.WrapHandler)
	}

	metrics := echo.WrapHandler(promhttp.Handler())
	switch {
	case obs.MetricsUser != "" && obs.MetricsPassword != "":
		e.GET("/metrics", metrics, metricsAuth(obs.MetricsUser, obs.MetricsPassword))
	case obs.MetricsUser != "" || obs.MetricsPassword != "":
		// half-set pair: fail closed
	case !cfg.IsProduction():
		e.GET("/metrics", metrics)
	}

	e.GET("/health", health.HealthCheck)
	e.GET("/ready", health.Readiness)
	e.GET("/live", health.Liveness)
}

// metricsAuth is Basic Auth for /metrics. Both fields are always compared
// (no short-circuit on the username), and each comparison is over fixed-length
// digests so neither value's length is observable.
func metricsAuth(user, pass string) echo.MiddlewareFunc {
	return middleware.BasicAuth(func(u, p string, _ echo.Context) (bool, error) {
		userOK := secureEqual(u, user)
		passOK := secureEqual(p, pass)
		return userOK && passOK, nil
	})
}

func secureEqual(a, b string) bool {
	ah, bh := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ah[:], bh[:]) == 1
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
	reportRepo := repository.NewReportRepo(db)
	auditRepo := repository.NewAuditRepo(db)

	// Services
	verificationService := service.NewVerificationService(userRepo, verificationRepo, log)
	authService := service.NewAuthService(userRepo, verificationService, log)
	passwordResetService := service.NewPasswordResetService(userRepo, passwordResetRepo, log)
	reportService := service.NewReportService(reportRepo, auditRepo, repository.NewTxManager(db.Pool), db.Pool, userRepo, roleRepo)

	// Handlers
	authHandler := handler.NewAuthHandler(authService, sm)
	userHandler := handler.NewUserHandler(userRepo)
	roleHandler := handler.NewRoleHandler(roleRepo, permissionRepo)
	permissionHandler := handler.NewPermissionHandler(permissionRepo)
	adminUserHandler := handler.NewAdminUserHandler(userRepo, roleRepo)
	verificationHandler := handler.NewVerificationHandler(verificationService)
	passwordResetHandler := handler.NewPasswordResetHandler(passwordResetService)
	reportHandler := handler.NewReportHandler(reportService, sm)
	healthHandler := handler.NewHealthHandler(db.Pool)

	e := newEcho(cfg, log)

	registerSystemRoutes(e, cfg, healthHandler)

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
		reportHandler,
		ourmiddleware.RateLimit(rate.Limit(float64(cfg.Reports.WriteRatePerMinute)/60), cfg.Reports.WriteBurst),
	)

	scsHandler := sm.LoadAndSave(e)

	server := newHTTPServer(cfg, scsHandler)

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
