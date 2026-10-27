package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

func serveListUsers(t *testing.T, repo *userimpl.MockStore, query string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.GET("/users", NewUserHandler(repo).ListUsers)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/users"+query, nil))
	return rec
}

func TestUserHandler_ListUsers_StatusFilter(t *testing.T) {
	var gotStatus string
	calls := 0
	repo := userimpl.NewMockStore()
	repo.ListFunc = func(_ context.Context, _ string, status string, _ int64, _ pagination.Params) ([]user.User, int64, error) {
		calls++
		gotStatus = status
		return []user.User{{ID: 1, Email: "a@example.com", Name: "A", Status: user.UserStatusInactive}}, 1, nil
	}

	for _, tt := range []struct{ query, want string }{
		{"", ""},
		{"?status=all", ""},
		{"?status=inactive", "inactive"},
		{"?status=disciplinary", "disciplinary"},
		// "deleted" is a list filter, not a status (D-A11); the store turns it into deleted_at IS NOT NULL.
		{"?status=deleted", "deleted"},
	} {
		calls = 0
		rec := serveListUsers(t, repo, tt.query)
		require.Equal(t, http.StatusOK, rec.Code, tt.query)
		assert.Equal(t, 1, calls, tt.query)
		assert.Equal(t, tt.want, gotStatus, tt.query)
		assert.Contains(t, rec.Body.String(), `"status":"inactive"`, "each user carries its status")
	}
}

func TestUserHandler_ListUsers_Pages(t *testing.T) {
	var got pagination.Params
	repo := userimpl.NewMockStore()
	repo.ListFunc = func(_ context.Context, _, _ string, _ int64, pg pagination.Params) ([]user.User, int64, error) {
		got = pg
		return []user.User{{ID: 1, Email: "a@example.com", Name: "A"}}, 45, nil
	}

	rec := serveListUsers(t, repo, "?page=3&limit=20")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, pagination.Params{Page: 3, Limit: 20}, got)
	assert.Contains(t, rec.Body.String(), `"meta":{"page":3,"limit":20,"total":45,"total_pages":3}`)
}

func TestUserHandler_ListUsers_UnknownStatusIs422(t *testing.T) {
	repo := userimpl.NewMockStore()
	repo.ListFunc = func(context.Context, string, string, int64, pagination.Params) ([]user.User, int64, error) {
		t.Error("the repository was called with an invalid status")
		return nil, 0, nil
	}

	for _, q := range []string{"?status=retired", "?status=bogus"} {
		rec := serveListUsers(t, repo, q)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, q)
		assert.Contains(t, rec.Body.String(), `"status"`, q)
	}
}
