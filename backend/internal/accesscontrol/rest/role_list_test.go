package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

type listRoleStore struct {
	accesscontrol.RoleStore
	roles              []accesscontrol.Role
	permissions        map[int64][]accesscontrol.Permission
	listErr, grantsErr error
	page, limit        int
	batchCalls         int
	singleCalls        int
	roleIDs            []int64
}

func (s *listRoleStore) ListWithPagination(_ context.Context, page, limit int) ([]accesscontrol.Role, int64, error) {
	s.page, s.limit = page, limit
	return s.roles, 7, s.listErr
}

func (s *listRoleStore) GetPermissions(context.Context, int64) ([]accesscontrol.Permission, error) {
	s.singleCalls++
	return nil, errors.New("unexpected per-role permissions lookup")
}

func (s *listRoleStore) GetPermissionsByRoleIDs(_ context.Context, ids []int64) (map[int64][]accesscontrol.Permission, error) {
	s.batchCalls++
	s.roleIDs = ids
	return s.permissions, s.grantsErr
}

func TestRoleHandler_List(t *testing.T) {
	read := accesscontrol.Permission{ID: 11, Name: "reports:read_any", Resource: "reports", Action: "read_any"}
	write := accesscontrol.Permission{ID: 12, Name: "users:write", Resource: "users", Action: "write"}
	tests := []struct {
		name        string
		store       listRoleStore
		wantStatus  int
		wantMessage string
		wantIDs     []int64
		wantData    []accesscontrol.RoleResponse
		wantBatch   int
	}{
		{
			name: "one lookup preserves role order, grants and counts",
			store: listRoleStore{
				roles: []accesscontrol.Role{
					{ID: 9, Name: "Reviewer", Description: "Reviews", UserCount: 2},
					{ID: 2, Name: "Editor", UserCount: 1},
					{ID: 5, Name: "No grants"},
				},
				permissions: map[int64][]accesscontrol.Permission{9: {read}, 2: {read, write}},
			},
			wantStatus: http.StatusOK, wantBatch: 1, wantIDs: []int64{9, 2, 5},
			wantData: []accesscontrol.RoleResponse{
				{ID: 9, Name: "Reviewer", Description: "Reviews", UserCount: 2, Permissions: []accesscontrol.PermissionResponse{{ID: 11, Name: "reports:read_any", Resource: "reports", Action: "read_any"}}},
				{ID: 2, Name: "Editor", UserCount: 1, Permissions: []accesscontrol.PermissionResponse{{ID: 11, Name: "reports:read_any", Resource: "reports", Action: "read_any"}, {ID: 12, Name: "users:write", Resource: "users", Action: "write"}}},
				{ID: 5, Name: "No grants"},
			},
		},
		{
			name: "empty page skips permission lookup", wantStatus: http.StatusOK,
			wantData: []accesscontrol.RoleResponse{},
		},
		{
			name:       "list failure skips permission lookup",
			store:      listRoleStore{listErr: errors.New("database down")},
			wantStatus: http.StatusInternalServerError, wantMessage: "failed to list roles",
		},
		{
			name:       "batch failure preserves permission error",
			store:      listRoleStore{roles: []accesscontrol.Role{{ID: 9}}, grantsErr: errors.New("database down")},
			wantStatus: http.StatusInternalServerError, wantMessage: "failed to load permissions",
			wantBatch: 1, wantIDs: []int64{9},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			e.HTTPErrorHandler = apperror.ErrorHandler
			e.GET("/admin/roles", NewRoleHandler(&tt.store, nil).List)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/roles?page=2&limit=3", nil))

			require.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, 2, tt.store.page)
			assert.Equal(t, 3, tt.store.limit)
			assert.Zero(t, tt.store.singleCalls, "list must not query permissions per role")
			assert.Equal(t, tt.wantBatch, tt.store.batchCalls)
			assert.Equal(t, tt.wantIDs, tt.store.roleIDs)
			if tt.wantStatus != http.StatusOK {
				var body errorBody
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, tt.wantMessage, body.Message)
				return
			}
			var body pagination.Response[accesscontrol.RoleResponse]
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tt.wantData, body.Data)
			assert.Equal(t, pagination.Meta{Page: 2, Limit: 3, Total: 7, TotalPages: 3}, body.Meta)
		})
	}
}
