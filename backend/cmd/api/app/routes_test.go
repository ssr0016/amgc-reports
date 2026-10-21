package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/config"
	"github.com/ssr0016/pastoral-reports/backend/internal/router"
)

const (
	metricsUser = "scraper"
	metricsPass = "a-long-enough-pass"
)

// systemEcho registers only the system routes. /health and /live never touch
// the pool, so no database is needed.
func systemEcho(stage config.Stage, obs config.ObservabilityConfig) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	cfg := &config.Config{Stage: stage, Observability: obs}
	registerSystemRoutes(e, cfg, router.NewHealthHandler(nil))
	return e
}

func get(e *echo.Echo, path string, auth ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	if len(auth) == 2 {
		req.SetBasicAuth(auth[0], auth[1])
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func creds() config.ObservabilityConfig {
	return config.ObservabilityConfig{MetricsUser: metricsUser, MetricsPassword: metricsPass}
}

// The Swagger UI is an API map. In production it is only there when asked for.
func TestSystemRoutes_ProdHidesSwaggerByDefault(t *testing.T) {
	e := systemEcho(config.StageProd, config.ObservabilityConfig{})

	if got := get(e, "/swagger/index.html").Code; got != http.StatusNotFound {
		t.Errorf("status = %d, want %d", got, http.StatusNotFound)
	}
}

func TestSystemRoutes_SwaggerEnabledOverridesProd(t *testing.T) {
	e := systemEcho(config.StageProd, config.ObservabilityConfig{SwaggerEnabled: true})

	if got := get(e, "/swagger/index.html").Code; got != http.StatusOK {
		t.Errorf("status = %d, want %d", got, http.StatusOK)
	}
}

func TestSystemRoutes_NonProdKeepsSwagger(t *testing.T) {
	e := systemEcho(config.StageLocal, config.ObservabilityConfig{SwaggerEnabled: true})

	if got := get(e, "/swagger/index.html").Code; got != http.StatusOK {
		t.Errorf("status = %d, want %d", got, http.StatusOK)
	}
}

// Request counts, latencies and error rates are not for the public.
func TestSystemRoutes_ProdHidesMetricsWithoutCredentials(t *testing.T) {
	e := systemEcho(config.StageProd, config.ObservabilityConfig{})

	if got := get(e, "/metrics").Code; got != http.StatusNotFound {
		t.Errorf("status = %d, want %d", got, http.StatusNotFound)
	}
}

func TestSystemRoutes_MetricsRequiresBasicAuth(t *testing.T) {
	e := systemEcho(config.StageProd, creds())

	tests := []struct {
		name string
		auth []string
		want int
	}{
		{"no credentials", nil, http.StatusUnauthorized},
		{"wrong password", []string{metricsUser, "not-the-password"}, http.StatusUnauthorized},
		{"wrong user", []string{"intruder", metricsPass}, http.StatusUnauthorized},
		{"correct credentials", []string{metricsUser, metricsPass}, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(e, "/metrics", tt.auth...)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			// The auth scheme is case-insensitive (RFC 7617); Echo sends "basic".
			if tt.want == http.StatusUnauthorized && !strings.HasPrefix(strings.ToLower(rec.Header().Get("WWW-Authenticate")), "basic") {
				t.Errorf("WWW-Authenticate = %q, want a Basic challenge", rec.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

func TestSystemRoutes_NonProdMetricsOpenWithoutCredentials(t *testing.T) {
	e := systemEcho(config.StageLocal, config.ObservabilityConfig{})

	if got := get(e, "/metrics").Code; got != http.StatusOK {
		t.Errorf("status = %d, want %d", got, http.StatusOK)
	}
}

// Configured credentials are never ignored, whatever the stage.
func TestSystemRoutes_NonProdMetricsStillAuthenticatedWhenCredentialsSet(t *testing.T) {
	e := systemEcho(config.StageLocal, creds())

	if got := get(e, "/metrics").Code; got != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", got, http.StatusUnauthorized)
	}
}

// Validate rejects a half-set pair at startup; a hand-built Config must not
// fail open either.
func TestSystemRoutes_MetricsHalfSetCredentialsIsNotRegistered(t *testing.T) {
	tests := []struct {
		name  string
		stage config.Stage
		obs   config.ObservabilityConfig
	}{
		{"prod, user only", config.StageProd, config.ObservabilityConfig{MetricsUser: metricsUser}},
		{"prod, password only", config.StageProd, config.ObservabilityConfig{MetricsPassword: metricsPass}},
		{"local, user only", config.StageLocal, config.ObservabilityConfig{MetricsUser: metricsUser}},
		{"local, password only", config.StageLocal, config.ObservabilityConfig{MetricsPassword: metricsPass}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := systemEcho(tt.stage, tt.obs)

			if got := get(e, "/metrics").Code; got != http.StatusNotFound {
				t.Errorf("status = %d, want %d", got, http.StatusNotFound)
			}
			if got := get(e, "/metrics", metricsUser, metricsPass).Code; got != http.StatusNotFound {
				t.Errorf("with credentials: status = %d, want %d", got, http.StatusNotFound)
			}
		})
	}
}

// Railway's healthcheck and load balancers must keep working.
func TestSystemRoutes_HealthAndLiveAlwaysPublic(t *testing.T) {
	e := systemEcho(config.StageProd, config.ObservabilityConfig{})

	for _, path := range []string{"/health", "/live"} {
		if got := get(e, path).Code; got != http.StatusOK {
			t.Errorf("GET %s: status = %d, want %d", path, got, http.StatusOK)
		}
	}
}
