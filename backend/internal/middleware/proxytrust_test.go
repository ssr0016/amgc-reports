package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func mustCIDRs(t *testing.T, cidrs ...string) []*net.IPNet {
	t.Helper()
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, s := range cidrs {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			t.Fatalf("ParseCIDR(%q): %v", s, err)
		}
		out = append(out, n)
	}
	return out
}

func requestFrom(remoteAddr string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	return req
}

// Only the configured ranges are trusted. Loopback, link-local and private
// peers are not, so a service on the private network cannot choose the client
// address by sending CF-Connecting-IP.
func TestTrustedProxyExtractor(t *testing.T) {
	railway := []string{"100.64.0.0/10"}
	// Production on Render: its local proxy connects over loopback (peer_ip=::1).
	render := []string{"::1/128"}

	tests := []struct {
		name       string
		trusted    []string
		remoteAddr string
		cfIP       string
		want       string
	}{
		{"Render loopback peer, client in CF-Connecting-IP", render, "[::1]:4000", "112.208.174.24", "112.208.174.24"},
		{"Render loopback peer, IPv6 client", render, "[::1]:4000", "2001:db8::1", "2001:db8::1"},
		{"Render loopback peer, no header", render, "[::1]:4000", "", "::1"},
		{"Render loopback peer, invalid header", render, "[::1]:4000", "not-an-ip", "::1"},
		{"Render loopback peer, list in header", render, "[::1]:4000", "1.2.3.4, 203.0.113.7", "::1"},
		{"trusted peer, valid header", railway, "100.64.0.7:4000", "203.0.113.7", "203.0.113.7"},
		{"last address of the range is trusted", railway, "100.127.255.255:4000", "203.0.113.7", "203.0.113.7"},
		{"just outside the range is not", railway, "100.128.0.1:4000", "203.0.113.7", "100.128.0.1"},
		{"public peer is not trusted", railway, "198.51.100.9:4000", "203.0.113.7", "198.51.100.9"},
		{"private peer is not trusted", railway, "10.0.0.5:4000", "203.0.113.7", "10.0.0.5"},
		{"loopback peer is not trusted", railway, "127.0.0.1:4000", "203.0.113.7", "127.0.0.1"},
		{"IPv6 loopback peer is not trusted", railway, "[::1]:4000", "203.0.113.7", "::1"},
		{"link-local peer is not trusted", railway, "169.254.1.1:4000", "203.0.113.7", "169.254.1.1"},
		{"IPv6 ULA peer is not trusted", railway, "[fd12::5]:4000", "203.0.113.7", "fd12::5"},
		{"IPv6 peer in a trusted IPv6 range", []string{"fd00::/8"}, "[fd00::5]:4000", "203.0.113.7", "203.0.113.7"},
		{"IPv6 peer outside the trusted IPv6 range", []string{"fd00::/8"}, "[2001:db8::5]:4000", "203.0.113.7", "2001:db8::5"},
		{"several ranges, second one matches", []string{"192.0.2.0/24", "::1/128"}, "[::1]:4000", "203.0.113.7", "203.0.113.7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extract := TrustedProxyExtractor(mustCIDRs(t, tt.trusted...))

			req := requestFrom(tt.remoteAddr)
			// Never read: Render appends its own hops to X-Forwarded-For, and a
			// client can set both. Neither may change the result.
			req.Header.Set(echo.HeaderXRealIP, "9.9.9.9")
			req.Header.Set(echo.HeaderXForwardedFor, "8.8.8.8, 10.24.225.5")
			if tt.cfIP != "" {
				req.Header.Set(ClientIPHeader, tt.cfIP)
			}

			if got := extract(req); got != tt.want {
				t.Errorf("extract() = %q, want %q", got, tt.want)
			}
		})
	}
}

// The extractor is installed on Echo, so everything built on c.RealIP() (the
// rate-limit key and the log's remote_ip) follows it.
func TestTrustedProxyExtractor_DrivesTheRateLimitKey(t *testing.T) {
	e := echo.New()
	e.IPExtractor = TrustedProxyExtractor(mustCIDRs(t, "::1/128"))

	keyFor := func(cfIP, forwarded string) string {
		req := requestFrom("[::1]:4000")
		req.Header.Set(ClientIPHeader, cfIP)
		req.Header.Set(echo.HeaderXForwardedFor, forwarded)
		return rateLimitKey(e.NewContext(req, httptest.NewRecorder()))
	}

	if a, b := keyFor("203.0.113.7", ""), keyFor("203.0.113.8", ""); a == b {
		t.Errorf("two clients behind one proxy share key %q", a)
	}
	if a, b := keyFor("203.0.113.7", "1.1.1.1"), keyFor("203.0.113.7", "2.2.2.2"); a != b {
		t.Errorf("rotating X-Forwarded-For changed the key: %q vs %q", a, b)
	}
	if got := keyFor("2001:db8:1:2::9", ""); got != "2001:db8:1:2::/64" {
		t.Errorf("IPv6 client key = %q, want its /64", got)
	}
}

type warnEntry struct {
	Level string   `json:"level"`
	Msg   string   `json:"msg"`
	CIDRs []string `json:"trusted_proxy_cidrs"`
}

func runWarnCheck(t *testing.T, trusted []string, window int, peers []string) []warnEntry {
	t.Helper()
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	e := echo.New()
	e.Use(WarnIfNoTrustedProxy(log, mustCIDRs(t, trusted...), window))
	e.GET("/", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	for _, peer := range peers {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, requestFrom(peer))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	}

	var entries []warnEntry
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var entry warnEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode %q: %v", line, err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func repeat(peer string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = peer
	}
	return out
}

func TestWarnIfNoTrustedProxy_WarnsOnceAfterTheWindow(t *testing.T) {
	railway := []string{"100.64.0.0/10"}

	t.Run("no trusted peer in the first 100 requests", func(t *testing.T) {
		entries := runWarnCheck(t, railway, TrustedProxyWarnWindow, repeat("198.51.100.9:4000", 250))

		if len(entries) != 1 {
			t.Fatalf("got %d log entries, want exactly 1: %+v", len(entries), entries)
		}
		got := entries[0]
		if got.Level != "WARN" {
			t.Errorf("level = %s, want WARN", got.Level)
		}
		want := "No request from a trusted proxy in the first 100 requests. " +
			"Check TRUSTED_PROXY_CIDRS — if wrong, all users share one bucket."
		if got.Msg != want {
			t.Errorf("msg = %q, want %q", got.Msg, want)
		}
		if len(got.CIDRs) != 1 || got.CIDRs[0] != "100.64.0.0/10" {
			t.Errorf("trusted_proxy_cidrs = %v, want [100.64.0.0/10]", got.CIDRs)
		}
	})

	t.Run("not before the window is full", func(t *testing.T) {
		entries := runWarnCheck(t, railway, TrustedProxyWarnWindow, repeat("198.51.100.9:4000", 99))
		if len(entries) != 0 {
			t.Errorf("warned after 99 requests: %+v", entries)
		}
	})

	t.Run("one trusted peer inside the window is enough", func(t *testing.T) {
		peers := append(repeat("198.51.100.9:4000", 40), "100.64.0.7:4000")
		peers = append(peers, repeat("198.51.100.9:4000", 200)...)

		if entries := runWarnCheck(t, railway, TrustedProxyWarnWindow, peers); len(entries) != 0 {
			t.Errorf("warned although a trusted peer was seen: %+v", entries)
		}
	})

	t.Run("a trusted peer after the window does not repeat the warning", func(t *testing.T) {
		peers := append(repeat("198.51.100.9:4000", 100), repeat("100.64.0.7:4000", 20)...)

		if entries := runWarnCheck(t, railway, TrustedProxyWarnWindow, peers); len(entries) != 1 {
			t.Errorf("got %d entries, want 1: %+v", len(entries), entries)
		}
	})

	t.Run("the window size is configurable", func(t *testing.T) {
		entries := runWarnCheck(t, railway, 3, repeat("198.51.100.9:4000", 3))
		if len(entries) != 1 || !strings.Contains(entries[0].Msg, "first 3 requests") {
			t.Errorf("entries = %+v, want one warning naming 3 requests", entries)
		}
	})
}
