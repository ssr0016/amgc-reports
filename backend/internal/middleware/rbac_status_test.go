package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

type allowAll struct{ accesscontrol.RoleStore }

func (allowAll) GetPermissions(context.Context, int64) ([]accesscontrol.Permission, error) {
	return []accesscontrol.Permission{{Name: "reports:read"}}, nil
}

// A session that outlives an account's status stops working: a non-active user is
// a 401 from LoadCaller in front of every gate, whatever role they hold.
func TestRBAC_NonActiveUserIsUnauthorized(t *testing.T) {
	tests := []struct {
		status string
		want   int
	}{
		{user.UserStatusActive, http.StatusOK},
		{"", http.StatusOK}, // an unset status only comes from a hand-built value
		{user.UserStatusInactive, http.StatusUnauthorized},
		{user.UserStatusDisciplinary, http.StatusUnauthorized},
		{user.UserStatusDeceased, http.StatusUnauthorized},
	}
	gates := map[string]echo.MiddlewareFunc{
		"RequireRole":       RequireRole("admin"),
		"RequirePermission": RequirePermission("reports:read"),
	}
	for name, gate := range gates {
		for _, tt := range tests {
			t.Run(name+"/"+tt.status, func(t *testing.T) {
				users := userimpl.NewMockStore()
				users.Roles = allowAll{}
				users.GetWithRoleFunc = func(context.Context, int64) (*user.User, error) {
					return &user.User{ID: 1, RoleID: 9, Role: &accesscontrol.Role{Name: "admin"}, Status: tt.status}, nil
				}
				e := echo.New()
				e.HTTPErrorHandler = apperror.ErrorHandler
				e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
					return func(c echo.Context) error {
						c.SetRequest(c.Request().WithContext(request.WithUser(c.Request().Context(), request.User{ID: 1})))
						return next(c)
					}
				})
				e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, LoadCaller(users), gate)

				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil))

				if rec.Code != tt.want {
					t.Errorf("status = %d, want %d", rec.Code, tt.want)
				}
			})
		}
	}
}
