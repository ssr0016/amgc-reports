package middleware

import (
	"strconv"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// MaxRequestBodyBytes caps every request body. All API payloads are small
// JSON documents, so 1 MiB is generous.
const MaxRequestBodyBytes = 1 << 20

// BodyLimit rejects request bodies larger than MaxRequestBodyBytes: with 413
// when Content-Length declares too much, and with a read error (surfaced by
// Bind as 400) when an undeclared or chunked body runs past the limit.
func BodyLimit() echo.MiddlewareFunc {
	return echomw.BodyLimitWithConfig(echomw.BodyLimitConfig{
		Limit: strconv.Itoa(MaxRequestBodyBytes) + "B",
	})
}
