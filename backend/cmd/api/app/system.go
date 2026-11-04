package app

import (
	"crypto/sha256"
	"crypto/subtle"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"

	_ "github.com/ssr0016/pastoral-reports/backend/docs"
	"github.com/ssr0016/pastoral-reports/backend/internal/config"
	"github.com/ssr0016/pastoral-reports/backend/internal/router"
	"github.com/ssr0016/pastoral-reports/backend/pkg/metrics"
)

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
func registerSystemRoutes(e *echo.Echo, cfg *config.Config, health *router.HealthHandler, pool *pgxpool.Pool) {
	obs := cfg.Observability

	if obs.SwaggerEnabled {
		e.GET("/swagger/*", echoSwagger.WrapHandler)
	}

	metricsHandler := echo.WrapHandler(metrics.Handler(pool))
	switch {
	case obs.MetricsUser != "" && obs.MetricsPassword != "":
		e.GET("/metrics", metricsHandler, metricsAuth(obs.MetricsUser, obs.MetricsPassword))
	case obs.MetricsUser != "" || obs.MetricsPassword != "":
		// half-set pair: fail closed
	case !cfg.IsProduction():
		e.GET("/metrics", metricsHandler)
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
