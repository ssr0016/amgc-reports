package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/time/rate"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol/acimpl"
	acrest "github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	auditrest "github.com/ssr0016/pastoral-reports/backend/internal/audit/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/authimpl"
	authrest "github.com/ssr0016/pastoral-reports/backend/internal/auth/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/token"
	"github.com/ssr0016/pastoral-reports/backend/internal/config"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/logger"
	ourmiddleware "github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/internal/report/reportimpl"
	reportrest "github.com/ssr0016/pastoral-reports/backend/internal/report/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/router"
	statsrest "github.com/ssr0016/pastoral-reports/backend/internal/stats/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats/statsimpl"
	userrest "github.com/ssr0016/pastoral-reports/backend/internal/user/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

// Per-process request limits for /api/v1. Reports writes are configured in
// cfg.Reports. shortcut: fixed values, move to env when they need tuning
// without a deploy; in-memory buckets, so move to a shared store before
// running more than one instance.
const (
	globalRatePerMinute = 120 // per client IP, all /api/v1 routes
	globalBurst         = 60
	readRatePerMinute   = 60 // per signed-in user, reports GET routes
	readBurst           = 20
)

func run() error {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if err := cfg.ValidateProxy(); err != nil {
		return fmt.Errorf("config validation: %w", err)
	}
	if err := cfg.ValidateAuth(); err != nil {
		return fmt.Errorf("config validation: %w", err)
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
		log.Warn("TRUSTED_PROXY_CIDRS not set, using default " + config.DefaultTrustedProxyCIDR +
			". Set it to the proxy range in front of the service.")
	}

	ctx := context.Background()
	db, err := database.NewWithPool(ctx, cfg.Database.URL, database.PoolSettings{
		MaxConns:        int32(cfg.Database.MaxConns), // #nosec G115 - validated to be small
		MinConns:        int32(cfg.Database.MinConns), // #nosec G115
		MaxConnLifetime: cfg.Database.ConnMaxLifetime,
		MaxConnIdleTime: cfg.Database.ConnMaxIdleTime,
	})
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

	tokens := token.New(token.Config{
		Secret:    []byte(cfg.Auth.JWTSecret),
		Issuer:    cfg.Auth.JWTIssuer,
		Audience:  cfg.Auth.JWTAudience,
		AccessTTL: cfg.Auth.AccessTTL,
	})

	// Repositories
	userRepo := userimpl.NewStore(db)
	roleRepo := acimpl.NewRoleStore(db)
	permissionRepo := acimpl.NewPermissionStore(db)
	verificationRepo := authimpl.NewVerificationStore(db)
	passwordResetRepo := authimpl.NewPasswordResetStore(db)
	reportRepo := reportimpl.NewStore(db)
	auditRepo := auditimpl.NewStore(db)
	refreshRepo := authimpl.NewRefreshStore(db)

	// Services
	verificationService := authimpl.NewVerificationService(userRepo, verificationRepo, log)
	authService := authimpl.NewService(userRepo, roleRepo, verificationService, auditRepo, db, log, tokens, refreshRepo, authimpl.TokenSettings{
		RefreshTTL: cfg.Auth.RefreshTTL, RefreshIdleTTL: cfg.Auth.RefreshIdleTTL,
	})
	passwordResetService := authimpl.NewPasswordResetService(userRepo, passwordResetRepo, log)
	reportService := reportimpl.NewService(reportRepo, auditRepo, db, userRepo, roleRepo)
	statsService := statsimpl.NewService(statsimpl.NewStore(db))
	userService := userimpl.NewService(userRepo, roleRepo, auditRepo, db)

	// Handlers
	authHandler := authrest.NewAuthHandler(authService, authrest.NewCookies(cfg.Auth, cfg.IsProduction()))
	userHandler := userrest.NewUserHandler(userRepo)
	roleHandler := acrest.NewRoleHandler(roleRepo, acimpl.NewService(roleRepo, permissionRepo, userRepo, auditRepo, db))
	permissionHandler := acrest.NewPermissionHandler(permissionRepo)
	adminUserHandler := userrest.NewAdminUserHandler(userRepo, userService)
	verificationHandler := authrest.NewVerificationHandler(verificationService)
	passwordResetHandler := authrest.NewPasswordResetHandler(passwordResetService)
	reportHandler := reportrest.NewReportHandler(reportService)
	statsHandler := statsrest.NewStatsHandler(statsService)
	auditHandler := auditrest.NewAuditHandler(auditimpl.NewService(auditimpl.NewReader(db)))
	healthHandler := router.NewHealthHandler(db.Pool)

	e := newEcho(cfg, log)

	registerSystemRoutes(e, cfg, healthHandler)

	// API routes
	router.Setup(
		e,
		ourmiddleware.Authenticate(tokens),
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
		statsHandler,
		auditHandler,
		router.Limiters{
			Global: ourmiddleware.RateLimit(rate.Limit(globalRatePerMinute/60.0), globalBurst),
			Read:   ourmiddleware.RateLimitByUser(rate.Limit(readRatePerMinute/60.0), readBurst),
			Write:  ourmiddleware.RateLimitByUser(rate.Limit(float64(cfg.Reports.WriteRatePerMinute)/60), cfg.Reports.WriteBurst),
		},
	)

	server := newHTTPServer(cfg, e)

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
