package router

import (
	"net/http"
	"testing"
)

// The Excel exports and their preview need the admin role and reports:read_any (D-A6, D-A15).
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
		{"no session", "admin", []string{"reports:read_any"}, "", http.StatusUnauthorized},
		{"editor holding reports:read_any is still not an admin", "editor", []string{"reports:read", "reports:read_any"}, "1", http.StatusForbidden},
		{"admin role without reports:read_any", "admin", []string{"reports:read"}, "1", http.StatusForbidden},
		{"admin with reports:read_any", "admin", []string{"reports:read_any"}, "1", http.StatusOK},
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
