package router

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
)

// The Excel exports and their preview need reports:export (D-A6, D-A15, 37b), not the admin role.
func TestAdminReportExportRoutes_Access(t *testing.T) {
	routes := []struct{ name, path, body string }{
		{"one", "/api/v1/admin/reports/1/excel", ""},
		{"month", "/api/v1/admin/reports/excel", `{"month":"May","year":2026}`},
		{"preview", "/api/v1/admin/reports/excel/preview", `{"report_id":1}`},
	}
	tests := []struct {
		name, role string
		perms      []string
		user       string // "" = no session
		want       int
	}{
		{"no session", "admin", []string{"reports:export"}, "", http.StatusUnauthorized},
		{"plain user who may read every report", "user", plainUserPerms, "1", http.StatusForbidden},
		{"editor holding reports:read_any", "editor", []string{"reports:read", "reports:read_any"}, "1", http.StatusForbidden},
		{"admin role without reports:export", "admin", []string{"reports:read_any"}, "1", http.StatusForbidden},
		{"custom role with reports:export", "Reviewer", reviewerPerms, "1", http.StatusOK},
		{"admin with reports:export", "admin", []string{"reports:export"}, "1", http.StatusOK},
	}
	for _, tt := range tests {
		for _, rt := range routes {
			t.Run(tt.name+"/"+rt.name, func(t *testing.T) {
				e, _ := adminUsersServer(tt.role, tt.perms...)
				if got := reportRequestAs(e, http.MethodPost, rt.path, rt.body, tt.user); got != tt.want {
					t.Errorf("status = %d, want %d", got, tt.want)
				}
			})
		}
	}
}

// M5: the downloads use their own bucket. A spent write bucket does not block them,
// and a spent export bucket does.
func TestAdminReportExportRoutes_UseExportLimiter(t *testing.T) {
	tooMany := func(echo.HandlerFunc) echo.HandlerFunc {
		return func(echo.Context) error { return apperror.RateLimit("slow down") }
	}
	routes := []struct{ name, path, body string }{
		{"one", "/api/v1/admin/reports/1/excel", ""},
		{"month", "/api/v1/admin/reports/excel", `{"month":"May","year":2026}`},
	}
	for _, rt := range routes {
		e, _ := adminUsersServerWith(Limiters{Global: noLimit, Read: noLimit, Write: tooMany, Export: noLimit}, "admin", "reports:export")
		if got := reportRequestAs(e, http.MethodPost, rt.path, rt.body, "1"); got != http.StatusOK {
			t.Errorf("%s with write bucket spent: status = %d, want 200", rt.name, got)
		}
		e, _ = adminUsersServerWith(Limiters{Global: noLimit, Read: noLimit, Write: noLimit, Export: tooMany}, "admin", "reports:export")
		if got := reportRequestAs(e, http.MethodPost, rt.path, rt.body, "1"); got != http.StatusTooManyRequests {
			t.Errorf("%s with export bucket spent: status = %d, want 429", rt.name, got)
		}
	}
}
