package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

// fakeRoleSvc answers every write with err, and records the last request.
type fakeRoleSvc struct {
	err     error
	created accesscontrol.CreateRoleRequest
	updated accesscontrol.UpdateRoleRequest
}

func (f *fakeRoleSvc) Create(_ context.Context, _ int64, _ string, req accesscontrol.CreateRoleRequest) (*accesscontrol.Role, error) {
	f.created = req
	if f.err != nil {
		return nil, f.err
	}
	return &accesscontrol.Role{ID: 4, Name: req.Name}, nil
}

func (f *fakeRoleSvc) Update(_ context.Context, _ int64, _ string, id int64, req accesscontrol.UpdateRoleRequest) (*accesscontrol.Role, error) {
	f.updated = req
	if f.err != nil {
		return nil, f.err
	}
	return &accesscontrol.Role{ID: id, Name: req.Name}, nil
}

func (f *fakeRoleSvc) Delete(context.Context, int64, string, int64) error { return f.err }

type errorBody struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Details map[string]string `json:"details"`
}

// serveAs runs one request through h as user 1.
func serveAs(t *testing.T, method, route string, h echo.HandlerFunc, target, body string) (int, errorBody) {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Validator = validator.New()
	e.Add(method, route, h)

	req := httptest.NewRequestWithContext(request.WithUser(context.Background(), request.User{ID: 1}), method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var out errorBody
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

func TestRoleHandler_Writes(t *testing.T) {
	const body = `{"name":"Reviewer","description":"Reads","permission_ids":[11]}`
	create := func(h *RoleHandler) (int, errorBody) {
		return serveAs(t, http.MethodPost, "/admin/roles", h.Create, "/admin/roles", body)
	}
	update := func(h *RoleHandler) (int, errorBody) {
		return serveAs(t, http.MethodPut, "/admin/roles/:id", h.Update, "/admin/roles/4", body)
	}
	del := func(h *RoleHandler) (int, errorBody) {
		return serveAs(t, http.MethodDelete, "/admin/roles/:id", h.Delete, "/admin/roles/4", "")
	}

	tests := []struct {
		name        string
		call        func(*RoleHandler) (int, errorBody)
		err         error
		wantStatus  int
		wantMessage string
	}{
		{"create", create, nil, http.StatusCreated, ""},
		{"update", update, nil, http.StatusOK, ""},
		{"delete", del, nil, http.StatusNoContent, ""},
		{"duplicate name on create", create, accesscontrol.ErrRoleNameTaken, http.StatusConflict, "role name already exists"},
		{"duplicate name on rename", update, accesscontrol.ErrRoleNameTaken, http.StatusConflict, "role name already exists"},
		{"built-in role update", update, fmt.Errorf("x: %w", accesscontrol.ErrProtectedRole), http.StatusConflict, "built-in roles cannot be changed"},
		{"built-in role delete", del, fmt.Errorf("x: %w", accesscontrol.ErrProtectedRole), http.StatusConflict, "built-in roles cannot be changed"},
		{"role in use", del, &accesscontrol.RoleInUseError{Users: 3}, http.StatusConflict, "role is assigned to 3 users; reassign them first"},
		{"role in use, count unknown", del, &accesscontrol.RoleInUseError{}, http.StatusConflict, "role is assigned to users; reassign them first"},
		{"missing role on update", update, accesscontrol.ErrRoleNotFound, http.StatusNotFound, "role not found"},
		{"missing role on delete", del, accesscontrol.ErrRoleNotFound, http.StatusNotFound, "role not found"},
		{"unknown permission", create, fmt.Errorf("x: %w", accesscontrol.ErrUnknownPermission), http.StatusUnprocessableEntity, "validation failed"},
		{"database failure stays 500", del, errors.New("connection refused"), http.StatusInternalServerError, "failed to delete role"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, out := tt.call(NewRoleHandler(nil, &fakeRoleSvc{err: tt.err}))
			if status != tt.wantStatus || out.Message != tt.wantMessage {
				t.Errorf("got %d %q, want %d %q", status, out.Message, tt.wantStatus, tt.wantMessage)
			}
		})
	}

	t.Run("unknown permission names the field", func(t *testing.T) {
		_, out := create(NewRoleHandler(nil, &fakeRoleSvc{err: accesscontrol.ErrUnknownPermission}))
		if out.Details["permission_ids"] == "" {
			t.Errorf("details = %v, want permission_ids", out.Details)
		}
	})
}

func TestRoleHandler_Validation(t *testing.T) {
	tests := []struct {
		name, method, body string
		wantField          string
	}{
		{"create needs a name", http.MethodPost, `{"permission_ids":[]}`, "name"},
		{"name of one character", http.MethodPost, `{"name":"x"}`, "name"},
		{"name over 50 characters", http.MethodPost, `{"name":"` + strings.Repeat("x", 51) + `"}`, "name"},
		{"description over 255", http.MethodPost, `{"name":"ok","description":"` + strings.Repeat("x", 256) + `"}`, "description"},
		{"repeated permission id", http.MethodPost, `{"name":"ok","permission_ids":[1,1]}`, "permission_ids"},
		{"update needs permission_ids", http.MethodPut, `{"name":"ok"}`, "permission_ids"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeRoleSvc{}
			h := NewRoleHandler(nil, svc)
			route, target, fn := "/admin/roles", "/admin/roles", h.Create
			if tt.method == http.MethodPut {
				route, target, fn = "/admin/roles/:id", "/admin/roles/4", h.Update
			}
			status, out := serveAs(t, tt.method, route, fn, target, tt.body)
			if status != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", status)
			}
			if _, ok := out.Details[tt.wantField]; !ok {
				t.Errorf("details = %v, want %s", out.Details, tt.wantField)
			}
		})
	}

	t.Run("update with an empty permission set is a full replace", func(t *testing.T) {
		svc := &fakeRoleSvc{}
		h := NewRoleHandler(nil, svc)
		status, _ := serveAs(t, http.MethodPut, "/admin/roles/:id", h.Update, "/admin/roles/4", `{"name":"ok","permission_ids":[]}`)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
		if svc.updated.PermissionIDs == nil || len(svc.updated.PermissionIDs) != 0 {
			t.Errorf("permission_ids = %v, want an empty set", svc.updated.PermissionIDs)
		}
	})
}
