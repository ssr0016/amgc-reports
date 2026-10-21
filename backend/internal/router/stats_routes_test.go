package router

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/handler"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
)

// okStatsSvc answers every call and counts them: a call proves a request got past the gates.
type okStatsSvc struct{ calls int }

func (s *okStatsSvc) Get(context.Context, *string, *int) (model.AdminStats, error) {
	s.calls++
	return model.AdminStats{Month: "September", Year: 2026}, nil
}

// statsServer wires the real routes. A request is user 1 when it carries
// X-Test-User; that user has the named role.
func statsServer(roleName string) (*echo.Echo, *okStatsSvc) {
	users := repository.NewMockUserRepo()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*model.User, error) {
		return &model.User{ID: id, RoleID: 9, Role: &model.Role{Name: roleName}}, nil
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
	Setup(e, noAuthn, users, permRoles{},
		handler.NewAuthHandler(nil, nil), nil, nil, nil, nil,
		handler.NewVerificationHandler(nil), handler.NewPasswordResetHandler(nil),
		nil, handler.NewStatsHandler(svc),
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
