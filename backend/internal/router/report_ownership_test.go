package router

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/handler"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

const (
	ownerUser  = "1" // plain user who files the report
	otherUser  = "2" // plain user, not the owner
	editorUser = "3" // holds the _any permissions
	adminUser  = "4" // holds every permission
)

// rolePerms maps a user ID to the role ID and permissions the fixtures give it.
var rolePerms = map[int64][]string{
	1: {"reports:read", "reports:write", "reports:delete"},
	2: {"reports:read", "reports:write", "reports:delete"},
	3: {"reports:read", "reports:write", "reports:delete", "reports:read_any", "reports:write_any", "reports:delete_any", "reports:approve"},
	4: {"reports:read", "reports:write", "reports:delete", "reports:read_any", "reports:write_any", "reports:delete_any", "reports:approve"},
}

// byRole serves permissions by role ID; a user's role ID is their user ID here.
type byRole struct{ repository.RoleRepository }

func (byRole) GetPermissions(_ context.Context, roleID int64) ([]model.Permission, error) {
	var out []model.Permission
	for _, n := range rolePerms[roleID] {
		out = append(out, model.Permission{Name: n})
	}
	return out, nil
}

// runTx runs fn directly: the in-memory mocks have no transaction to roll back.
type runTx struct{}

func (runTx) WithTx(_ context.Context, fn func(repository.Querier) error) error { return fn(nil) }

// ownershipServer wires the real routes, handler and ReportService over in-memory
// repositories, so the ownership rule is exercised through HTTP. Unlike
// reportsServer it has no stub service.
func ownershipServer() *echo.Echo {
	sm := scs.New()
	users := repository.NewMockUserRepo()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*model.User, error) {
		return &model.User{ID: id, RoleID: id, Role: &model.Role{Name: "test"}}, nil
	}
	svc := service.NewReportService(repository.NewMockReportRepo(), repository.NewMockAuditRepo(), runTx{}, nil, users, byRole{})

	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(echo.WrapMiddleware(sm.LoadAndSave))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if id, err := strconv.ParseInt(c.Request().Header.Get("X-Test-User"), 10, 64); err == nil {
				sm.Put(c.Request().Context(), "user_id", id)
			}
			return next(c)
		}
	})
	Setup(e, sm, users, byRole{},
		handler.NewAuthHandler(nil, nil), nil, nil, nil, nil,
		handler.NewVerificationHandler(nil), handler.NewPasswordResetHandler(nil),
		handler.NewReportHandler(svc, sm),
		Limiters{Global: noLimit, Read: noLimit, Write: noLimit},
	)
	return e
}

// Another user's report is a 404, never a 403, so its existence is not disclosed (D-1).
// The _any permissions (editor, admin) reach it; the owner always can.
func TestReportsOwnership(t *testing.T) {
	const report = "/api/v1/reports/1"
	body := `{"month":"May"}`
	cases := []struct {
		name, user, method, body string
		want                     int
	}{
		{"non-owner PUT", otherUser, http.MethodPut, body, http.StatusNotFound},
		{"non-owner DELETE", otherUser, http.MethodDelete, "", http.StatusNotFound},
		{"non-owner GET", otherUser, http.MethodGet, "", http.StatusNotFound},
		{"editor PUT", editorUser, http.MethodPut, body, http.StatusOK},
		{"editor DELETE", editorUser, http.MethodDelete, "", http.StatusNoContent},
		{"admin PUT", adminUser, http.MethodPut, body, http.StatusOK},
		{"owner PUT", ownerUser, http.MethodPut, body, http.StatusOK},
		{"owner DELETE", ownerUser, http.MethodDelete, "", http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := ownershipServer()
			if got := reportRequestAs(e, http.MethodPost, "/api/v1/reports", body, ownerUser); got != http.StatusCreated {
				t.Fatalf("seed POST: status = %d, want 201", got)
			}
			if got := reportRequestAs(e, tc.method, report, tc.body, tc.user); got != tc.want {
				t.Errorf("status = %d, want %d", got, tc.want)
			}
		})
	}
}
