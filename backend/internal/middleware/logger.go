package middleware

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/ssr0016/template/internal/logger"
)

// SlogLogger returns an Echo middleware that logs requests using slog.
func SlogLogger(log *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			reqID := c.Response().Header().Get(echo.HeaderXRequestID)
			if reqID == "" {
				reqID = c.Request().Header.Get(echo.HeaderXRequestID)
			}

			ctx := logger.WithRequestIDContext(c.Request().Context(), reqID)
			reqLog := log.With("request_id", reqID)
			ctx = logger.WithContext(ctx, reqLog)
			c.SetRequest(c.Request().WithContext(ctx))

			err := next(c)

			// Determine actual status code
			status := c.Response().Status
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					status = he.Code
				}
			}

			attrs := []any{
				"method", c.Request().Method,
				"uri", c.Request().RequestURI,
				"status", status,
				"latency_ms", time.Since(start).Milliseconds(),
				"remote_ip", c.RealIP(),
				"user_agent", c.Request().UserAgent(),
				"bytes_in", c.Request().ContentLength,
				"bytes_out", c.Response().Size,
			}

			if err != nil {
				attrs = append(attrs, "error", err.Error())
			}

			// Log level based on STATUS CODE (not just error presence)
			switch {
			case status >= 500:
				reqLog.Error("server error", attrs...)
			case status >= 400:
				reqLog.Warn("client error", attrs...)
			default:
				reqLog.Info("request completed", attrs...)
			}

			return err
		}
	}
}
