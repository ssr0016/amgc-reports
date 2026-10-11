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
				"remote_ip", normalizeClientIP(c),
				"peer_ip", peerIP(c),
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

const (
	redactedValue   = "[REDACTED]"
	malformedMarker = "[MALFORMED]"
)

// redactURI returns uri in a form that is safe to log. The query string is
// handled pair by pair:
//
//   - a pair whose decoded key is sensitive keeps its raw key and has its
//     value replaced by [REDACTED];
//   - a pair that net/url would refuse (it contains ";" or an invalid percent
//     escape) is replaced whole by [MALFORMED]; Echo's QueryParam skips such
//     pairs, so they cannot carry a token to a handler;
//   - every other pair, including empty ones, is kept byte for byte.
//
// Values are never decoded. Pairs are handled independently so that one bad
// pair cannot hide the rest of the query from the log.
func redactURI(uri string) string {
	path, query, found := strings.Cut(uri, "?")
	if !found || query == "" {
		return uri
	}

	pairs := strings.Split(query, "&")
	for i, pair := range pairs {
		pairs[i] = redactPair(pair)
	}
	return path + "?" + strings.Join(pairs, "&")
}

func redactPair(pair string) string {
	if pair == "" {
		return pair
	}
	if strings.Contains(pair, ";") || !validEscapes(pair) {
		return malformedMarker
	}

	rawKey, _, hasValue := strings.Cut(pair, "=")
	if !hasValue {
		return pair
	}
	key, err := url.QueryUnescape(rawKey)
	if err != nil {
		return malformedMarker // unreachable after validEscapes; fail closed anyway
	}
	if isSensitiveQueryParam(key) {
		return rawKey + "=" + redactedValue
	}
	return pair
}

// validEscapes reports whether every "%" in s starts a two-digit hex escape.
// It only checks; it never decodes.
func validEscapes(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+2 >= len(s) || !isHex(s[i+1]) || !isHex(s[i+2]) {
			return false
		}
		i += 2
	}
	return true
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func isSensitiveQueryParam(name string) bool {
	for _, s := range sensitiveQueryParams {
		if strings.EqualFold(name, s) {
			return true
		}
	}
	return false
}
