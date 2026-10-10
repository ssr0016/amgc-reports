package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/template/internal/apperror"
	"github.com/ssr0016/template/internal/model"
	"github.com/ssr0016/template/internal/repository"
)

type AdminUserHandler struct {
	userRepo repository.UserRepository
	roleRepo repository.RoleRepository
}

func NewAdminUserHandler(userRepo repository.UserRepository, roleRepo repository.RoleRepository) *AdminUserHandler {
	return &AdminUserHandler{
		userRepo: userRepo,
		roleRepo: roleRepo,
	}
}

// GetUser godoc
// @Summary      Get user with role
// @Tags         admin/users
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "User ID"
// @Success      200 {object} model.UserResponse
// @Router       /admin/users/{id} [get]
func (h *AdminUserHandler) GetUser(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperror.BadRequest("invalid user id")
	}

	user, err := h.userRepo.GetWithRole(c.Request().Context(), id)
	if err != nil {
		return apperror.Internal("failed to get user").WithError(err)
	}
	if user == nil {
		return apperror.NotFound("user not found")
	}
	return c.JSON(http.StatusOK, user.ToResponse())
}

// ChangeRole godoc
// @Summary      Change a user's role
// @Tags         admin/users
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "User ID"
// @Param        body body model.ChangeRoleRequest true "Role change payload"
// @Success      200 {object} model.UserResponse
// @Router       /admin/users/{id}/role [put]
func (h *AdminUserHandler) ChangeRole(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperror.BadRequest("invalid user id")
	}

	var req model.ChangeRoleRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return apperror.Validation(err.Error())
	}

	// Verify user exists
	user, err := h.userRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return apperror.Internal("failed to get user").WithError(err)
	}
	if user == nil {
		return apperror.NotFound("user not found")
	}

	// Verify role exists
	role, err := h.roleRepo.GetByID(c.Request().Context(), req.RoleID)
	if err != nil {
		return apperror.Internal("failed to get role").WithError(err)
	}
	if role == nil {
		return apperror.NotFound("role not found")
	}

	// Update user's role
	if err := h.userRepo.UpdateRole(c.Request().Context(), id, req.RoleID); err != nil {
		return apperror.Internal("failed to update role").WithError(err)
	}

	// Reload user with role
	updatedUser, err := h.userRepo.GetWithRole(c.Request().Context(), id)
	if err != nil {
		return apperror.Internal("failed to reload user").WithError(err)
	}

	return c.JSON(http.StatusOK, updatedUser.ToResponse())
}
