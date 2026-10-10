package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/template/internal/apperror"
	"github.com/ssr0016/template/internal/model"
	"github.com/ssr0016/template/internal/repository"
)

type UserHandler struct {
	repo repository.UserRepository
}

func NewUserHandler(repo repository.UserRepository) *UserHandler {
	return &UserHandler{repo: repo}
}

// ListUsers godoc
// @Summary      List users
// @Description  Returns paginated list of users
// @Tags         users
// @Produce      json
// @Security     CookieAuth
// @Param        page query int false "Page number (default 1)"
// @Param        limit query int false "Limit (default 20, max 100)"
// @Param        email query string false "Filter by email"
// @Success      200 {array} model.UserResponse
// @Failure      401 {object} map[string]string
// @Router       /users [get]
func (h *UserHandler) ListUsers(c echo.Context) error {
	emailFilter := c.QueryParam("email")
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	users, err := h.repo.List(c.Request().Context(), emailFilter, limit)
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
// @Description  Returns a single user
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
