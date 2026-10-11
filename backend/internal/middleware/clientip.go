package middleware

import (
	"net"
	"net/netip"
	"strings"

	"github.com/labstack/echo/v4"
)

// unknownClient is the key used when no client address can be determined.
const unknownClient = "unknown"

// ipv6KeyPrefixBits is how much of an IPv6 address identifies a client for
// rate limiting. A host normally controls a whole /64, so keying by the full
// address would let it rotate through 2^64 buckets.
const ipv6KeyPrefixBits = 64

// clientAddr returns the client's address, parsed and canonical: an IPv6 zone
// is dropped and an IPv4-mapped IPv6 address is unmapped, so one client has
// one address. It is c.RealIP() when that is an IP address, otherwise the
// connection's own address; ok is false when neither is.
//
// c.RealIP() cannot be used directly as a key or in logs. It returns whatever
// the client put in X-Forwarded-For or X-Real-IP, and when the header is a
// comma-separated list the result is a substring of it, so even a short valid
// IP keeps the whole header value alive in the limiter's map. Parsing the value
// and re-rendering it allocates a fresh string of at most 39 bytes.
//
// Parsing does not make the value trustworthy. Which peer's X-Real-IP is
// believed is decided by the IPExtractor installed on Echo, see
// TrustedProxyExtractor; without one a client can still choose any valid IP.
func clientAddr(c echo.Context) (netip.Addr, bool) {
	if addr, ok := parseAddr(c.RealIP()); ok {
		return addr, true
	}
	return parseAddr(peerHost(c))
}

// normalizeClientIP returns the client's full canonical address, for logs. It
// is "unknown" when there is none. Use rateLimitKey to key a limiter.
func normalizeClientIP(c echo.Context) string {
	if addr, ok := clientAddr(c); ok {
		return addr.String()
	}
	return unknownClient
}

// rateLimitKey returns the key under which a client is rate limited: the
// address itself for IPv4, the /64 prefix (for example "2001:db8:1:2::/64")
// for IPv6. The suffix keeps a prefix key from colliding with an IPv4 key or
// with "unknown". It is built from clientAddr, so it shares the log's parsing.
func rateLimitKey(c echo.Context) string {
	addr, ok := clientAddr(c)
	if !ok {
		return unknownClient
	}
	if addr.Is6() {
		return netip.PrefixFrom(addr, ipv6KeyPrefixBits).Masked().String()
	}
	return addr.String()
}

// peerIP returns the address of the TCP peer that opened the connection, in
// canonical form. No request header can influence it. Behind a reverse proxy
// it is the proxy's address, which is what makes it useful in logs: it shows
// which peer is allowed to speak for clients (see TrustedProxyExtractor). It is
// "unknown" when RemoteAddr is not an IP address.
func peerIP(c echo.Context) string {
	if addr, ok := parseAddr(peerHost(c)); ok {
		return addr.String()
	}
	return unknownClient
}

// peerHost is the host part of the connection's RemoteAddr.
func peerHost(c echo.Context) string {
	remote := c.Request().RemoteAddr
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return remote
	}
	return host
}

func parseAddr(s string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(s))
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.WithZone("").Unmap(), true
}
