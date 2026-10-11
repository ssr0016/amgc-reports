package middleware

import (
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"

	"github.com/labstack/echo/v4"
)

// TrustedProxyWarnWindow is how many requests WarnIfNoTrustedProxy watches
// before concluding that no trusted proxy is connecting.
const TrustedProxyWarnWindow = 100

// TrustedProxyExtractor returns an Echo IP extractor that believes X-Real-IP
// only when the TCP peer is inside one of the trusted ranges, and otherwise
// uses the peer itself. X-Forwarded-For is never read.
//
// Trust is exactly the configured ranges: loopback, link-local and private
// peers are not trusted unless a range covers them, so another service on a
// private network cannot choose the client address.
//
// If the ranges are wrong the extractor fails closed to the peer's address,
// which behind a proxy is the proxy's: every user then shares one rate-limit
// bucket. WarnIfNoTrustedProxy exists to make that visible.
func TrustedProxyExtractor(trusted []*net.IPNet) echo.IPExtractor {
	opts := []echo.TrustOption{
		echo.TrustLoopback(false),
		echo.TrustLinkLocal(false),
		echo.TrustPrivateNet(false),
	}
	for _, n := range trusted {
		opts = append(opts, echo.TrustIPRange(n))
	}
	return echo.ExtractIPFromRealIPHeader(opts...)
}

// WarnIfNoTrustedProxy logs one warning when none of the first window requests
// came from a trusted peer. After that, or after the first trusted peer, it
// does nothing.
func WarnIfNoTrustedProxy(log *slog.Logger, trusted []*net.IPNet, window int) echo.MiddlewareFunc {
	cidrs := make([]string, 0, len(trusted))
	for _, n := range trusted {
		cidrs = append(cidrs, n.String())
	}

	var (
		seen atomic.Int64
		done atomic.Bool
	)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !done.Load() {
				switch {
				case peerIsTrusted(c, trusted):
					done.Store(true)
				case seen.Add(1) >= int64(window) && done.CompareAndSwap(false, true):
					log.Warn(noTrustedProxyMessage(window), "trusted_proxy_cidrs", cidrs)
				}
			}
			return next(c)
		}
	}
}

func noTrustedProxyMessage(window int) string {
	return fmt.Sprintf("No request from a trusted proxy in the first %d requests. "+
		"Check TRUSTED_PROXY_CIDRS — if wrong, all users share one bucket.", window)
}

func peerIsTrusted(c echo.Context, trusted []*net.IPNet) bool {
	addr, ok := parseAddr(peerHost(c))
	if !ok {
		return false
	}
	ip := net.IP(addr.AsSlice())
	for _, n := range trusted {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
