package handler

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

type UserHandler struct {
	repo repository.UserRepository
}

func NewUserHandler(repo repository.UserRepository) *UserHandler {
	return &UserHandler{repo: repo}
}

// ListUsers godoc
// @Summary      List users
// @Description  Admin only. Returns paginated list of users
// @Tags         users
// @Produce      json
// @Security     CookieAuth
// @Param        page query int false "Page number (default 1)"
// @Param        limit query int false "Limit (default 20, max 100)"
// @Param        email query string false "Filter by email"
// @Param        status query string false "Filter by status; all or empty means every status" Enums(all, active, inactive, disciplinary, deceased)
// @Success      200 {array} model.UserResponse
// @Failure      401 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /users [get]
func (h *UserHandler) ListUsers(c echo.Context) error {
	emailFilter := c.QueryParam("email")
	status := c.QueryParam("status")
	if status == "all" {
		status = ""
	}
	if status != "" && !slices.Contains(model.UserStatuses, status) {
		return apperror.Validation("invalid status").WithDetails(map[string]string{"status": "invalid status"})
	}
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	users, err := h.repo.List(c.Request().Context(), emailFilter, status, limit)
	if err != nil {
		return apperror.Internal("failed to list users").WithError(err)
	}

	out := make([]model.UserResponse, 0, len(users))
	for i := range users {
		out = append(out, users[i].ToResponse())
	}
	return c.JSON(http.StatusOK, out)
}

// GetUser godoc
// @Summary      Get user by ID
// @Description  Admin only. Returns a single user
// @Tags         users
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "User ID"
// @Success      200 {object} model.UserResponse
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
