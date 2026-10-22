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
	Setup(e, noAuthn, users, permRoles{perms: perms},
		authrest.NewAuthHandler(nil, nil), nil, nil, nil, nil,
		authrest.NewVerificationHandler(nil), authrest.NewPasswordResetHandler(nil),
		nil, statsrest.NewStatsHandler(svc), nil,
		Limiters{Global: noLimit, Read: noLimit, Write: noLimit},
	)
	return e, svc
}

// The counts are admin-only: the role gates the route. An editor, who may read every report, still gets 403.
func TestStatsRoute_AdminOnly(t *testing.T) {
	tests := []struct {
		name, role, user string // user "" = no session
		want             int
		reaches          bool
	}{
		{"no session", "admin", "", http.StatusUnauthorized, false},
		{"plain user", "user", "1", http.StatusForbidden, false},
		{"editor", "editor", "1", http.StatusForbidden, false},
		{"admin", "admin", "1", http.StatusOK, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, svc := statsServer(tt.role)
			if got := reportRequestAs(e, http.MethodGet, "/api/v1/admin/stats", "", tt.user); got != tt.want {
				t.Fatalf("status = %d, want %d", got, tt.want)
			}
			if (svc.calls == 1) != tt.reaches {
				t.Errorf("service calls = %d: only an admin may reach it", svc.calls)
			}
		})
	}
}

// The tracker needs the admin role and reports:read_any: names and report status, but no report text.
func TestTrackerRoute_AdminWithReadAny(t *testing.T) {
	tests := []struct {
		name, role, user string // user "" = no session
		perms            []string
		want             int
	}{
		{"no session", "admin", "", []string{"reports:read_any"}, http.StatusUnauthorized},
		{"plain user", "user", "1", nil, http.StatusForbidden},
		{"editor who may read every report", "editor", "1", []string{"reports:read_any"}, http.StatusForbidden},
		{"admin without reports:read_any", "admin", "1", nil, http.StatusForbidden},
		{"admin", "admin", "1", []string{"reports:read_any"}, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, svc := statsServer(tt.role, tt.perms...)
			if got := reportRequestAs(e, http.MethodGet, "/api/v1/admin/tracker", "", tt.user); got != tt.want {
				t.Fatalf("status = %d, want %d", got, tt.want)
			}
			if (svc.calls == 1) != (tt.want == http.StatusOK) {
				t.Errorf("service calls = %d", svc.calls)
			}
		})
	}
}
