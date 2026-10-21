package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ssr0016/pastoral-reports/backend/internal/config"
)

const wantMaxHeaderBytes = 16 * 1024

func testServerConfig() *config.Config {
	return &config.Config{App: config.AppConfig{Port: "8080"}}
}

func TestNewHTTPServer_SetsMaxHeaderBytes(t *testing.T) {
	srv := newHTTPServer(testServerConfig(), http.NotFoundHandler())

	if srv.MaxHeaderBytes != wantMaxHeaderBytes {
		t.Errorf("MaxHeaderBytes = %d, want %d", srv.MaxHeaderBytes, wantMaxHeaderBytes)
	}
}

// net/http answers 431 itself, before any handler or Echo middleware runs.
func TestNewHTTPServer_RejectsOversizeHeaders_431(t *testing.T) {
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ts.Config = newHTTPServer(testServerConfig(), ts.Config.Handler)
	ts.Start()
	defer ts.Close()

	tests := []struct {
		name      string
		headerLen int
		want      int
	}{
		{"small headers pass", 1024, http.StatusOK},
		{"headers far over the limit", 64 * 1024, http.StatusRequestHeaderFieldsTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("X-Padding", strings.Repeat("a", tt.headerLen))

			resp, err := ts.Client().Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != tt.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.want)
			}
		})
	}
}
