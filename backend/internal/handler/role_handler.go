package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/template/internal/apperror"
	"github.com/ssr0016/template/internal/model"
	"github.com/ssr0016/template/internal/repository"
	"github.com/ssr0016/template/pkg/pagination"
)

type RoleHandler struct {
	roleRepo       repository.RoleRepository
	permissionRepo repository.PermissionRepository
}

func NewRoleHandler(roleRepo repository.RoleRepository, permissionRepo repository.PermissionRepository) *RoleHandler {
	return &RoleHandler{
		roleRepo:       roleRepo,
		permissionRepo: permissionRepo,
	}
}

// List godoc
// @Summary      List all roles
// @Tags         admin/roles
// @Produce      json
// @Security     CookieAuth
// @Success      200 {array} model.RoleResponse
// @Router       /admin/roles [get]
func (h *RoleHandler) List(c echo.Context) error {
	params := pagination.FromContext(c)

	roles, total, err := h.roleRepo.ListWithPagination(
		c.Request().Context(),
		params.Page,
		params.Limit,
	)
	if err != nil {
		return apperror.Internal("failed to list roles").WithError(err)
	}

	// Load permissions for each role
	out := make([]model.RoleResponse, 0, len(roles))
	for i := range roles {
		perms, err := h.roleRepo.GetPermissions(c.Request().Context(), roles[i].ID)
		if err != nil {
			return apperror.Internal("failed to load permissions").WithError(err)
		}
		roles[i].Permissions = perms
		out = append(out, roles[i].ToResponse())
	}

	return c.JSON(http.StatusOK, pagination.NewResponse(out, params, total))
}

// Get godoc
// @Summary      Get role by ID
// @Tags         admin/roles
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "Role ID"
// @Success      200 {object} model.RoleResponse
// @Router       /admin/roles/{id} [get]
func (h *RoleHandler) Get(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperror.BadRequest("invalid role id")
	}

	role, err := h.roleRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return apperror.Internal("failed to get role").WithError(err)
	}
	if role == nil {
		return apperror.NotFound("role not found")
	}

	perms, err := h.roleRepo.GetPermissions(c.Request().Context(), role.ID)
	if err != nil {
		return apperror.Internal("failed to load permissions").WithError(err)
	}
	role.Permissions = perms

	return c.JSON(http.StatusOK, role.ToResponse())
}

// Create godoc
// @Summary      Create a new role
// @Tags         admin/roles
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        body body model.CreateRoleRequest true "Role payload"
// @Success      201 {object} model.RoleResponse
// @Router       /admin/roles [post]
func (h *RoleHandler) Create(c echo.Context) error {
	var req model.CreateRoleRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return apperror.Validation(err.Error())
	}

	// Check if role name already exists
	existing, err := h.roleRepo.GetByName(c.Request().Context(), req.Name)
	if err != nil {
		return apperror.Internal("failed to check role").WithError(err)
	}
	if existing != nil {
		return apperror.Conflict("role name already exists")
	}

	role, err := h.roleRepo.Create(c.Request().Context(), req)
	if err != nil {
		return apperror.Internal("failed to create role").WithError(err)
	}
	return c.JSON(http.StatusCreated, role.ToResponse())
}

// Update godoc
// @Summary      Update a role
// @Tags         admin/roles
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "Role ID"
// @Param        body body model.UpdateRoleRequest true "Role update payload"
// @Success      200 {object} model.RoleResponse
// @Router       /admin/roles/{id} [put]
func (h *RoleHandler) Update(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperror.BadRequest("invalid role id")
	}

	var req model.UpdateRoleRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return apperror.Validation(err.Error())
	}

	role, err := h.roleRepo.Update(c.Request().Context(), id, req)
	if err != nil {
		return apperror.Internal("failed to update role").WithError(err)
	}
	if role == nil {
		return apperror.NotFound("role not found")
	}

	perms, err := h.roleRepo.GetPermissions(c.Request().Context(), role.ID)
	if err != nil {
		return apperror.Internal("failed to load permissions").WithError(err)
	}
	role.Permissions = perms

	return c.JSON(http.StatusOK, role.ToResponse())
}

// Delete godoc
// @Summary      Delete a role
// @Tags         admin/roles
// @Security     CookieAuth
// @Param        id path int true "Role ID"
// @Success      204
// @Router       /admin/roles/{id} [delete]
func (h *RoleHandler) Delete(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperror.BadRequest("invalid role id")
	}

	if err := h.roleRepo.Delete(c.Request().Context(), id); err != nil {
		return apperror.Internal("failed to delete role").WithError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// AssignPermission godoc
// @Summary      Assign a permission to a role
// @Tags         admin/roles
// @Accept       json
// @Security     CookieAuth
// @Param        id path int true "Role ID"
// @Param        body body model.AssignPermissionRequest true "Permission payload"
// @Success      200 {object} map[string]string
// @Router       /admin/roles/{id}/permissions [post]
func (h *RoleHandler) AssignPermission(c echo.Context) error {
	roleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperror.BadRequest("invalid role id")
	}

	var req model.AssignPermissionRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return apperror.Validation(err.Error())
	}

	// Verify role exists
	role, err := h.roleRepo.GetByID(c.Request().Context(), roleID)
	if err != nil {
		return apperror.Internal("failed to get role").WithError(err)
	}
	if role == nil {
		return apperror.NotFound("role not found")
	}

	// Verify permission exists
	perm, err := h.permissionRepo.GetByID(c.Request().Context(), req.PermissionID)
	if err != nil {
		return apperror.Internal("failed to get permission").WithError(err)
	}
	if perm == nil {
		return apperror.NotFound("permission not found")
	}

	if err := h.roleRepo.AssignPermission(c.Request().Context(), roleID, req.PermissionID); err != nil {
		return apperror.Internal("failed to assign permission").WithError(err)
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "permission assigned"})
}

// RevokePermission godoc
// @Summary      Revoke a permission from a role
// @Tags         admin/roles
// @Security     CookieAuth
// @Param        id path int true "Role ID"
// @Param        pid path int true "Permission ID"
// @Success      204
// @Router       /admin/roles/{id}/permissions/{pid} [delete]
func (h *RoleHandler) RevokePermission(c echo.Context) error {
	roleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperror.BadRequest("invalid role id")
	}
	permID, err := strconv.ParseInt(c.Param("pid"), 10, 64)
	if err != nil {
		return apperror.BadRequest("invalid permission id")
	}

	if err := h.roleRepo.RevokePermission(c.Request().Context(), roleID, permID); err != nil {
		return apperror.Internal("failed to revoke permission").WithError(err)
	}
	return c.NoContent(http.StatusNoContent)
}
