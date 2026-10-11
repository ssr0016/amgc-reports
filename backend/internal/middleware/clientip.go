package middleware

import (
	"net"
	"net/netip"
	"strings"

	"github.com/labstack/echo/v4"
)

// unknownClient is the key used when no client address can be determined.
const unknownClient = "unknown"

// normalizeClientIP returns the client's address as a short canonical string,
// for use as a rate-limit key and in logs.
//
// c.RealIP() cannot be used directly for either. It returns whatever the
// client put in X-Forwarded-For or X-Real-IP, and when the header is a
// comma-separated list the result is a substring of it, so even a short valid
// IP keeps the whole header value alive in the limiter's map. The value is
// parsed as an IP address and re-rendered, which allocates a fresh string of
// at most 39 bytes. An IPv6 zone is dropped and an IPv4-mapped IPv6 address
// is unmapped, so one client has one key. A value that is not an IP address
// is replaced by the connection's own address.
//
// Parsing does not make the header trustworthy: a client can still choose any
// valid IP. That is H-4.
func normalizeClientIP(c echo.Context) string {
	if addr, ok := canonicalAddr(c.RealIP()); ok {
		return addr
	}

	host, _, err := net.SplitHostPort(c.Request().RemoteAddr)
	if err != nil {
		host = c.Request().RemoteAddr
	}
	if addr, ok := canonicalAddr(host); ok {
		return addr
	}
	return unknownClient
}

func canonicalAddr(s string) (string, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(s))
	if err != nil {
		return "", false
	}
	return addr.WithZone("").Unmap().String(), true
}
