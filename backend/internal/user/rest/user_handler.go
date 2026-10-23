package rest

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

type UserHandler struct {
	repo user.Store
}

func NewUserHandler(repo user.Store) *UserHandler {
	return &UserHandler{repo: repo}
}

// ListUsers godoc
// @Summary      List users
// @Description  Needs users:list. Returns paginated list of users
// @Tags         users
// @Produce      json
// @Security     CookieAuth
// @Param        page query int false "Page number (default 1)"
// @Param        limit query int false "Limit (default 20, max 100)"
// @Param        email query string false "Filter by email"
// @Param        role_id query int false "Filter by role ID"
// @Param        status query string false "Filter by status; all or empty means every status. deleted returns only deleted users; every other value leaves them out" Enums(all, active, inactive, disciplinary, deceased, deleted)
// @Success      200 {array} user.UserResponse
// @Failure      401 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /users [get]
func (h *UserHandler) ListUsers(c echo.Context) error {
	emailFilter := c.QueryParam("email")
	status := c.QueryParam("status")
	if status == "all" {
		status = ""
	}
	if status != "" && status != "deleted" && !slices.Contains(user.UserStatuses, status) {
		return apperror.Validation("invalid status").WithDetails(map[string]string{"status": "invalid status"})
	}
	var roleID int64
	if r := c.QueryParam("role_id"); r != "" {
		var err error
		if roleID, err = strconv.ParseInt(r, 10, 64); err != nil || roleID <= 0 {
			return apperror.Validation("invalid role_id").WithDetails(map[string]string{"role_id": "invalid role id"})
		}
	}
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	users, err := h.repo.List(c.Request().Context(), emailFilter, status, roleID, limit)
	if err != nil {
		return apperror.Internal("failed to list users").WithError(err)
	}

	out := make([]user.UserResponse, 0, len(users))
	for i := range users {
		out = append(out, users[i].ToResponse())
	}
	return c.JSON(http.StatusOK, out)
}

// GetUser godoc
// @Summary      Get user by ID
// @Description  Needs users:list. Returns a single user
// @Tags         users
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "User ID"
// @Success      200 {object} user.UserResponse
// @Failure      401 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /users/{id} [get]
func (h *UserHandler) GetUser(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperror.BadRequest("invalid user id")
	}

	user, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return apperror.Internal("failed to get user").WithError(err)
	}
	if user == nil {
		return apperror.NotFound("user not found")
	}
	return c.JSON(http.StatusOK, user.ToResponse())
}
