package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/pkg/metrics"
)

const (
	// unmatchedPath labels requests that matched no route.
	unmatchedPath = "unmatched"
	// otherMethod labels request methods outside the standard set.
	otherMethod = "OTHER"
)

// methodLabel returns m when it is a standard HTTP method and otherMethod
// otherwise: the method token is client-chosen, so it must not mint series.
func methodLabel(m string) string {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodConnect,
		http.MethodOptions, http.MethodTrace:
		return m
	}
	return otherMethod
}

// PrometheusMiddleware captures HTTP metrics for Prometheus.
func PrometheusMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Track in-flight requests
			metrics.HTTPRequestsInFlight.Inc()
			defer metrics.HTTPRequestsInFlight.Dec()

			start := time.Now()

			// Call next handler
			err := next(c)

			// Calculate duration
			duration := time.Since(start).Seconds()

			// Get path pattern. The raw URL path is client-chosen, so an
			// unmatched request must not become its own series.
			path := c.Path()
			if path == "" {
				path = unmatchedPath
			}
			method := methodLabel(c.Request().Method)

			// Get status code
			status := c.Response().Status
			if err != nil && !c.Response().Committed {
				status = apperror.StatusOf(err)
			}

			// Record metrics
			metrics.HTTPRequestsTotal.WithLabelValues(
				method,
				path,
				strconv.Itoa(status),
			).Inc()

			metrics.HTTPRequestDuration.WithLabelValues(
				method,
				path,
			).Observe(duration)

			return err
		}
	}
}
