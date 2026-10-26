package router

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	authrest "github.com/ssr0016/pastoral-reports/backend/internal/auth/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
	statsrest "github.com/ssr0016/pastoral-reports/backend/internal/stats/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

// okStatsSvc answers every call and counts them: a call proves a request got past the gates.
type okStatsSvc struct{ calls int }

func (s *okStatsSvc) Get(context.Context, stats.GetStatsQuery) (stats.AdminStats, error) {
	s.calls++
	return stats.AdminStats{Month: "September", Year: 2026}, nil
}

func (s *okStatsSvc) Tracker(context.Context, stats.GetStatsQuery) (stats.Tracker, error) {
	s.calls++
	return stats.Tracker{Month: "September", Year: 2026, Rows: []stats.TrackerRow{}}, nil
}

func (s *okStatsSvc) AnalyticsAttendance(context.Context, stats.AttendanceQuery) (stats.AttendanceAnalytics, error) {
	s.calls++
	return stats.AttendanceAnalytics{}, nil
}

// statsServer wires the real routes. A request is user 1 when it carries
// X-Test-User; that user has the named role and permissions.
func statsServer(roleName string, perms ...string) (*echo.Echo, *okStatsSvc) {
	users := userimpl.NewMockStore()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*user.User, error) {
		return &user.User{ID: id, RoleID: 9, Role: &accesscontrol.Role{Name: roleName}}, nil
	}
	svc := &okStatsSvc{}
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if id, err := strconv.ParseInt(c.Request().Header.Get("X-Test-User"), 10, 64); err == nil {
				c.SetRequest(c.Request().WithContext(request.WithUser(c.Request().Context(), request.User{ID: id})))
			}
			return next(c)
		}
	})
	users.Roles = permRoles{perms: perms}
	Setup(e, noAuthn, users,
		authrest.NewAuthHandler(nil, nil), nil, nil, nil, nil, nil,
		authrest.NewVerificationHandler(nil), authrest.NewPasswordResetHandler(nil),
		nil, statsrest.NewStatsHandler(svc), nil,
		Limiters{Global: noLimit, Read: noLimit, Write: noLimit},
	)
	return e, svc
}

// The counts and the Tracker need tracker:read (37b), not the admin role. The
// plain user and the editor, who may read every report, get 403.
func TestStatsAndTrackerRoutes_NeedTrackerRead(t *testing.T) {
	tests := []struct {
		name, role, user string // user "" = no session
		perms            []string
		want             int
	}{
		{"no session", "admin", "", []string{"tracker:read"}, http.StatusUnauthorized},
		{"plain user", "user", "1", plainUserPerms, http.StatusForbidden},
		{"editor who may read every report", "editor", "1", []string{"reports:read", "reports:read_any"}, http.StatusForbidden},
		{"admin role without tracker:read", "admin", "1", []string{"reports:read_any"}, http.StatusForbidden},
		{"custom role with tracker:read", "Reviewer", "1", reviewerPerms, http.StatusOK},
		{"admin", "admin", "1", []string{"tracker:read"}, http.StatusOK},
	}
	for _, tt := range tests {
		for _, path := range []string{"/api/v1/admin/stats", "/api/v1/admin/tracker", "/api/v1/admin/analytics/attendance?year=2026&metric=worship_service"} {
			t.Run(tt.name+path, func(t *testing.T) {
				e, svc := statsServer(tt.role, tt.perms...)
				if got := reportRequestAs(e, http.MethodGet, path, "", tt.user); got != tt.want {
					t.Fatalf("status = %d, want %d", got, tt.want)
				}
				if (svc.calls == 1) != (tt.want == http.StatusOK) {
					t.Errorf("service calls = %d", svc.calls)
				}
			})
		}
	}
}
