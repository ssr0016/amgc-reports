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
