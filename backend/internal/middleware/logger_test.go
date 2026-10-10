package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
)

func TestSlogLogger_LevelAndStatus(t *testing.T) {
	tests := []struct {
		name       string
		handler    echo.HandlerFunc
		wantStatus int
		wantLevel  string
	}{
		{"401 warns", func(echo.Context) error { return apperror.Unauthorized("no") }, http.StatusUnauthorized, "WARN"},
		{"403 warns", func(echo.Context) error { return apperror.Forbidden("no") }, http.StatusForbidden, "WARN"},
		{"422 warns", func(echo.Context) error { return apperror.Validation("bad") }, http.StatusUnprocessableEntity, "WARN"},
		{"429 warns", func(echo.Context) error { return apperror.RateLimit("slow") }, http.StatusTooManyRequests, "WARN"},
		{"500 errors", func(echo.Context) error { return errors.New("boom") }, http.StatusInternalServerError, "ERROR"},
		{"200 infos", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, http.StatusOK, "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, nil))

			e := echo.New()
			e.HTTPErrorHandler = apperror.ErrorHandler
			e.Use(SlogLogger(log))
			e.GET("/l", tt.handler)

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/l", nil))

			var entry struct {
				Level  string `json:"level"`
				Status int    `json:"status"`
			}
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("decode log line %q: %v", buf.String(), err)
			}
			if entry.Level != tt.wantLevel {
				t.Errorf("level = %s, want %s", entry.Level, tt.wantLevel)
			}
			if entry.Status != tt.wantStatus {
				t.Errorf("logged status = %d, want %d", entry.Status, tt.wantStatus)
			}
		})
	}
}
