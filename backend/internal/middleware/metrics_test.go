package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/pkg/metrics"
)

func newMetricsEcho() *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(PrometheusMiddleware())
	return e
}

func requestCount(path string, status int) float64 {
	return testutil.ToFloat64(
		metrics.HTTPRequestsTotal.WithLabelValues(http.MethodGet, path, strconv.Itoa(status)),
	)
}

func TestPrometheusMiddleware_RecordsActualStatus(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		handler    echo.HandlerFunc
		wantStatus int
	}{
		{"401", "/m/401", func(echo.Context) error { return apperror.Unauthorized("no") }, http.StatusUnauthorized},
		{"403", "/m/403", func(echo.Context) error { return apperror.Forbidden("no") }, http.StatusForbidden},
		{"422", "/m/422", func(echo.Context) error { return apperror.Validation("bad") }, http.StatusUnprocessableEntity},
		{"429", "/m/429", func(echo.Context) error { return apperror.RateLimit("slow") }, http.StatusTooManyRequests},
		{"500 unknown error", "/m/500", func(echo.Context) error { return errors.New("boom") }, http.StatusInternalServerError},
		{"404 echo http error", "/m/404", func(echo.Context) error { return echo.NewHTTPError(http.StatusNotFound) }, http.StatusNotFound},
		{"200 success", "/m/200", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newMetricsEcho()
			e.GET(tt.path, tt.handler)

			before := requestCount(tt.path, tt.wantStatus)
			wrong200 := requestCount(tt.path, http.StatusOK)

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("response code = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := requestCount(tt.path, tt.wantStatus) - before; got != 1 {
				t.Errorf("http_requests_total{status=%d} delta = %v, want 1", tt.wantStatus, got)
			}
			if tt.wantStatus != http.StatusOK {
				if got := requestCount(tt.path, http.StatusOK) - wrong200; got != 0 {
					t.Errorf("http_requests_total{status=200} delta = %v, want 0", got)
				}
			}
		})
	}
}

func TestPrometheusMiddleware_CommittedThenError(t *testing.T) {
	const path = "/m/committed"
	e := newMetricsEcho()
	e.GET(path, func(c echo.Context) error {
		if err := c.NoContent(http.StatusAccepted); err != nil {
			return err
		}
		return errors.New("late failure") // ErrorHandler skips: response already on the wire
	})

	before := requestCount(path, http.StatusAccepted)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("response code = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if got := requestCount(path, http.StatusAccepted) - before; got != 1 {
		t.Errorf("http_requests_total{status=202} delta = %v, want 1", got)
	}
	if got := requestCount(path, http.StatusInternalServerError); got != 0 {
		t.Errorf("http_requests_total{status=500} = %v, want 0", got)
	}
}
