package middleware

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/template/pkg/metrics"
)

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

			// Get path pattern (para hindi mag-explode ang cardinality)
			path := c.Path()
			if path == "" {
				path = c.Request().URL.Path
			}

			// Get status code
			status := c.Response().Status
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					status = he.Code
				}
			}

			// Record metrics
			metrics.HTTPRequestsTotal.WithLabelValues(
				c.Request().Method,
				path,
				strconv.Itoa(status),
			).Inc()

			metrics.HTTPRequestDuration.WithLabelValues(
				c.Request().Method,
				path,
			).Observe(duration)

			return err
		}
	}
}
