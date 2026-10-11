package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/labstack/echo/v4"
)

// remoteHost is the host part of httptest's default RemoteAddr (192.0.2.1:1234).
const remoteHost = "192.0.2.1"

func contextWithForwardedFor(value string) echo.Context {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	if value != "" {
		req.Header.Set(echo.HeaderXForwardedFor, value)
	}
	return echo.New().NewContext(req, httptest.NewRecorder())
}

// viewsInto reports whether key's bytes live inside host's backing array.
// Go strings are immutable, so mutating the header cannot reveal this; the
// address range can.
func viewsInto(key, host string) bool {
	if key == "" || host == "" {
		return false
	}
	k := uintptr(unsafe.Pointer(unsafe.StringData(key)))  //nolint:gosec // test-only address comparison
	h := uintptr(unsafe.Pointer(unsafe.StringData(host))) //nolint:gosec // test-only address comparison
	return k >= h && k < h+uintptr(len(host))
}

// short keeps failure output readable when a key is a 100 KiB header.
func short(s string) string {
	if len(s) > 48 {
		return fmt.Sprintf("%s... (%d bytes)", s[:48], len(s))
	}
	return s
}

func heapAlloc() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

func TestNormalizeClientIP_PaddedValidIPIsNotAViewOfTheHeader(t *testing.T) {
	header := "203.0.113.7, " + strings.Repeat("x", 100<<10)

	key := normalizeClientIP(contextWithForwardedFor(header))

	if key != "203.0.113.7" {
		t.Fatalf("key = %q, want 203.0.113.7", short(key))
	}
	if len(key) > 45 {
		t.Errorf("key is %d bytes, an IP never needs more than 45", len(key))
	}
	if viewsInto(key, header) {
		t.Error("key is a substring of the request header, so it pins the whole header value in memory")
	}
}

func TestNormalizeClientIP_UnparseableValuesFallBackToTheConnection(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{"no header", ""},
		{"a word", "not-an-ip"},
		{"junk before a valid IP", "garbage, 203.0.113.7"},
		{"huge junk", strings.Repeat("x", 100<<10)},
		{"IP with a port", "203.0.113.7:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeClientIP(contextWithForwardedFor(tt.header)); got != remoteHost {
				t.Errorf("key = %q, want the connection host %q", short(got), remoteHost)
			}
		})
	}
}

// Padding after the comma must cost nothing: with the raw RealIP() as the key
// every visitor retained its whole 100 KiB header.
func TestNormalizeClientIP_HugeHeadersAreNotRetained(t *testing.T) {
	const n = 200
	pad := strings.Repeat("x", 100<<10)
	keys := make([]string, 0, n)

	before := heapAlloc()
	for i := 0; i < n; i++ {
		value := fmt.Sprintf("10.0.%d.%d, ", i/250, i%250) + pad
		keys = append(keys, normalizeClientIP(contextWithForwardedFor(value)))
	}
	after := heapAlloc()
	runtime.KeepAlive(keys)

	retained := int64(after) - int64(before)
	if retained > 1<<20 {
		t.Errorf("%d keys retain %d KiB; padded headers are being kept alive (unfixed it is about %d KiB)",
			n, retained>>10, n*100)
	}
}

func TestNormalizeClientIP_Canonicalizes(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"IPv6 loopback, short form", "::1", "::1"},
		{"IPv6 loopback, long form", "0:0:0:0:0:0:0:1", "::1"},
		{"IPv6 upper case", "2001:DB8::1", "2001:db8::1"},
		{"IPv4-mapped IPv6", "::ffff:203.0.113.7", "203.0.113.7"},
		{"IPv6 zone is dropped", "fe80::1%eth0", "fe80::1"},
		{"a huge zone is dropped too", "fe80::1%" + strings.Repeat("z", 1000), "fe80::1"},
		{"surrounding spaces", "  203.0.113.7  ", "203.0.113.7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeClientIP(contextWithForwardedFor(tt.header)); got != tt.want {
				t.Errorf("key = %q, want %q", short(got), tt.want)
			}
		})
	}
}

// peerIP is the address of the TCP peer, which no header can influence. It is
// what lets an operator see which proxy actually connects to the app.
func TestPeerIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{"IPv4 with port", "192.0.2.1:1234", "192.0.2.1"},
		{"IPv6 with port", "[2001:db8::1]:443", "2001:db8::1"},
		{"IPv6 non-canonical", "[2001:DB8:0:0:0:0:0:1]:443", "2001:db8::1"},
		{"IPv4-mapped IPv6", "[::ffff:100.64.0.7]:80", "100.64.0.7"},
		{"no port", "192.0.2.1", "192.0.2.1"},
		{"not an address", "unix-socket", unknownClient},
		{"empty", "", unknownClient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			c := echo.New().NewContext(req, httptest.NewRecorder())

			if got := peerIP(c); got != tt.want {
				t.Errorf("peerIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPeerIP_IgnoresForwardingHeaders(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "100.64.0.7:4000"
	req.Header.Set(echo.HeaderXForwardedFor, "203.0.113.9")
	req.Header.Set(echo.HeaderXRealIP, "198.51.100.23")
	c := echo.New().NewContext(req, httptest.NewRecorder())

	if got := peerIP(c); got != "100.64.0.7" {
		t.Errorf("peerIP() = %q, want the connection's 100.64.0.7", got)
	}
}

// rateLimitKey identifies a client for rate limiting. An IPv6 client normally
// controls a whole /64, so the key is the /64 prefix: rotating the low 64 bits
// buys nothing. IPv4 addresses are keyed whole.
func TestRateLimitKey(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		want       string
	}{
		{"IPv4 is keyed whole", "", "203.0.113.7", "203.0.113.7"},
		{"IPv6 is keyed by its /64", "", "2001:db8:1:2::1", "2001:db8:1:2::/64"},
		{"all-ones low half, same /64", "", "2001:db8:1:2:ffff:ffff:ffff:ffff", "2001:db8:1:2::/64"},
		{"first bit after the prefix, same /64", "", "2001:db8:1:2:8000::1", "2001:db8:1:2::/64"},
		{"last bit of the prefix, next /64", "", "2001:db8:1:3::1", "2001:db8:1:3::/64"},
		{"upper case is canonicalized", "", "2001:DB8:1:2::1", "2001:db8:1:2::/64"},
		{"IPv4-mapped IPv6 is keyed as IPv4", "", "::ffff:203.0.113.7", "203.0.113.7"},
		{"zone is dropped", "", "fe80::1%eth0", "fe80::/64"},
		{"junk header falls back to an IPv4 peer", "", "not-an-ip", remoteHost},
		{"junk header falls back to an IPv6 peer, grouped", "[2001:db8:9:9::5]:443", "not-an-ip", "2001:db8:9:9::/64"},
		{"no usable address at all", "unix-socket", "", unknownClient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := contextWithForwardedFor(tt.forwarded)
			if tt.remoteAddr != "" {
				c.Request().RemoteAddr = tt.remoteAddr
			}

			if got := rateLimitKey(c); got != tt.want {
				t.Errorf("rateLimitKey() = %q, want %q", short(got), tt.want)
			}
		})
	}
}

// The key shape is what keeps the visitor cap meaningful, and a prefix key
// must never collide with an IPv4 key or the "unknown" bucket.
func TestRateLimitKey_IsShortAndDistinctFromOtherKinds(t *testing.T) {
	header := "2001:db8:1:2::1, " + strings.Repeat("x", 100<<10)

	key := rateLimitKey(contextWithForwardedFor(header))

	if len(key) > 43 {
		t.Errorf("key is %d bytes, want at most 43", len(key))
	}
	if viewsInto(key, header) {
		t.Error("key is a view of the request header")
	}
	if key == unknownClient || !strings.HasSuffix(key, "/64") {
		t.Errorf("key = %q, want a /64 prefix", key)
	}
}

// Logs keep the exact address; only the limiter groups by prefix.
func TestNormalizeClientIP_KeepsTheFullIPv6Address(t *testing.T) {
	got := normalizeClientIP(contextWithForwardedFor("2001:DB8:1:2::abcd"))

	if got != "2001:db8:1:2::abcd" {
		t.Errorf("normalizeClientIP() = %q, want the full canonical address", got)
	}
}
