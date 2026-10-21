package app

import (
	"log/slog"
	"net/http"

	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/config"
	ourmiddleware "github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

// newEcho builds the Echo instance with its global middleware chain and no
// routes, so the chain can be tested without a database.
//
// BodyLimit sits after the logger and Prometheus on purpose: its check is only
// a Content-Length comparison, so it is cheap there, and a request it rejects
// still reaches the access log and http_requests_total.
func newEcho(cfg *config.Config, log *slog.Logger) *echo.Echo {
	e := echo.New()
	e.IPExtractor = ourmiddleware.TrustedProxyExtractor(cfg.Proxy.TrustedCIDRs)
	e.Validator = validator.New()
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		// Recover routes panics here too, so each 5xx is reported once; the captured stack still holds the panic site.
		if hub := sentryecho.GetHubFromContext(c); hub != nil && apperror.StatusOf(err) >= http.StatusInternalServerError {
			hub.CaptureException(err)
		}
		apperror.ErrorHandler(err, c)
	}
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.RequestID())
	e.Use(ourmiddleware.WarnIfNoTrustedProxy(log, cfg.Proxy.TrustedCIDRs, ourmiddleware.TrustedProxyWarnWindow))
	e.Use(ourmiddleware.SlogLogger(log))
	e.Use(ourmiddleware.PrometheusMiddleware())
	e.Use(ourmiddleware.BodyLimit())
	e.Use(ourmiddleware.CSRFProtection())
	// Before Recover, so it never sees a panic: it only adds the request's hub and transaction.
	e.Use(sentryecho.New(sentryecho.Options{}))
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     cfg.CORS.AllowedOrigins,
		AllowCredentials: true,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, echo.HeaderXCSRFToken},
		// Browsers hide non-safelisted response headers from scripts, so the
		// frontend could not read the wait time on a 429.
		ExposeHeaders: []string{echo.HeaderRetryAfter},
	}))
	return e
}
