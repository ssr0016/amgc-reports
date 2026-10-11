package middleware

import (
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/logger"
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
			if err != nil && !c.Response().Committed {
				status = apperror.StatusOf(err)
			}

			attrs := []any{
				"method", c.Request().Method,
				"uri", redactURI(c.Request().RequestURI),
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

// sensitiveQueryParams lists query parameter names whose values must never
// reach the logs. Names are compared case-insensitively after URL-decoding.
// To protect another parameter, add its name here.
var sensitiveQueryParams = []string{"token"}

const redactedValue = "[REDACTED]"

// redactURI returns uri in a form that is safe to log: the value of every
// sensitive query parameter is replaced by [REDACTED] and the rest of the URI
// is kept byte for byte.
//
// A query string that net/url cannot parse is dropped entirely. Echo's
// QueryParam silently skips bad pairs, so a malformed-looking query can still
// carry a working token; failing closed is the only safe answer.
func redactURI(uri string) string {
	path, query, found := strings.Cut(uri, "?")
	if !found || query == "" {
		return uri
	}
	if _, err := url.ParseQuery(query); err != nil {
		return path
	}

	pairs := strings.Split(query, "&")
	for i, pair := range pairs {
		rawKey, _, hasValue := strings.Cut(pair, "=")
		if !hasValue {
			continue
		}
		// An unescape failure cannot happen after ParseQuery succeeded; treat
		// it as sensitive anyway.
		key, err := url.QueryUnescape(rawKey)
		if err != nil || isSensitiveQueryParam(key) {
			pairs[i] = rawKey + "=" + redactedValue
		}
	}
	return path + "?" + strings.Join(pairs, "&")
}

func isSensitiveQueryParam(name string) bool {
	for _, s := range sensitiveQueryParams {
		if strings.EqualFold(name, s) {
			return true
		}
	}
	return false
}
