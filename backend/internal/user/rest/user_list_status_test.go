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
	repo.ListFunc = func(_ context.Context, _ string, status string, _ int) ([]user.User, error) {
		calls++
		gotStatus = status
		return []user.User{{ID: 1, Email: "a@example.com", Name: "A", Status: user.UserStatusInactive}}, nil
	}

	for _, tt := range []struct{ query, want string }{
		{"", ""},
		{"?status=all", ""},
		{"?status=inactive", "inactive"},
		{"?status=disciplinary", "disciplinary"},
	} {
		calls = 0
		rec := serveListUsers(t, repo, tt.query)
		require.Equal(t, http.StatusOK, rec.Code, tt.query)
		assert.Equal(t, 1, calls, tt.query)
		assert.Equal(t, tt.want, gotStatus, tt.query)
		assert.Contains(t, rec.Body.String(), `"status":"inactive"`, "each user carries its status")
	}
}

func TestUserHandler_ListUsers_UnknownStatusIs422(t *testing.T) {
	repo := userimpl.NewMockStore()
	repo.ListFunc = func(context.Context, string, string, int) ([]user.User, error) {
		t.Error("the repository was called with an invalid status")
		return nil, nil
	}

	rec := serveListUsers(t, repo, "?status=retired")

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status"`)
}
