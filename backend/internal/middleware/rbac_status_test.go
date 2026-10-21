package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
)

type allowAll struct{ repository.RoleRepository }

func (allowAll) GetPermissions(context.Context, int64) ([]model.Permission, error) {
	return []model.Permission{{Name: "reports:read"}}, nil
}

// A session that outlives an account's status stops working: a non-active user is
// a 401 at every gate that loads the user, whatever role they hold.
func TestRBAC_NonActiveUserIsUnauthorized(t *testing.T) {
	tests := []struct {
		status string
		want   int
	}{
		{model.UserStatusActive, http.StatusOK},
		{"", http.StatusOK}, // an unset status only comes from a hand-built value
		{model.UserStatusInactive, http.StatusUnauthorized},
		{model.UserStatusDisciplinary, http.StatusUnauthorized},
		{model.UserStatusDeceased, http.StatusUnauthorized},
	}
	gates := map[string]func(repository.UserRepository) echo.MiddlewareFunc{
		"RequireRole": func(u repository.UserRepository) echo.MiddlewareFunc {
			return RequireRole(u, "admin")
		},
		"RequirePermission": func(u repository.UserRepository) echo.MiddlewareFunc {
			return RequirePermission(allowAll{}, u, "reports:read")
		},
	}
	for name, gate := range gates {
		for _, tt := range tests {
			t.Run(name+"/"+tt.status, func(t *testing.T) {
				users := repository.NewMockUserRepo()
				users.GetWithRoleFunc = func(context.Context, int64) (*model.User, error) {
					return &model.User{ID: 1, RoleID: 9, Role: &model.Role{Name: "admin"}, Status: tt.status}, nil
				}
				e := echo.New()
				e.HTTPErrorHandler = apperror.ErrorHandler
				e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
					return func(c echo.Context) error {
						c.SetRequest(c.Request().WithContext(request.WithUser(c.Request().Context(), request.User{ID: 1})))
						return next(c)
					}
				})
				e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, gate(users))

				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil))

				if rec.Code != tt.want {
					t.Errorf("status = %d, want %d", rec.Code, tt.want)
				}
			})
		}
	}
}
