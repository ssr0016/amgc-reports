package rest

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// RoleHandler serves roles. Reads call the store; writes go through the service,
// which audits them (D-A21).
type RoleHandler struct {
	roleRepo accesscontrol.RoleStore
	svc      accesscontrol.RoleService
}

func NewRoleHandler(roleRepo accesscontrol.RoleStore, svc accesscontrol.RoleService) *RoleHandler {
	return &RoleHandler{roleRepo: roleRepo, svc: svc}
}

// roleError maps service errors to API errors; anything unexpected is a 500
// that carries no detail.
func roleError(err error, msg string) error {
	var inUse *accesscontrol.RoleInUseError
	switch {
	case errors.As(err, &inUse) && inUse.Users > 0:
		return apperror.Conflict(fmt.Sprintf("role is assigned to %d users; reassign them first", inUse.Users))
	case errors.Is(err, accesscontrol.ErrRoleInUse):
		return apperror.Conflict("role is assigned to users; reassign them first")
	case errors.Is(err, accesscontrol.ErrRoleNameTaken):
		return apperror.Conflict("role name already exists")
	case errors.Is(err, accesscontrol.ErrProtectedRole):
		return apperror.Conflict("built-in roles cannot be changed")
	case errors.Is(err, accesscontrol.ErrRoleNotFound):
		return apperror.NotFound("role not found")
	case errors.Is(err, accesscontrol.ErrUnknownPermission):
		return apperror.Validation("validation failed").
			WithDetails(map[string]string{"permission_ids": "unknown permission"})
	case errors.Is(err, auth.ErrUnauthorized):
		return apperror.Unauthorized("authentication required")
	}
	return apperror.Internal(msg).WithError(err)
}

func actorID(c echo.Context) (int64, error) {
	u, ok := request.UserFrom(c.Request().Context())
	if !ok {
		return 0, apperror.Unauthorized("authentication required")
	}
	return u.ID, nil
}

// List godoc
// @Summary      List all roles
// @Description  Admin only. Each role carries its permissions and user_count: every account holding it, deleted ones included.
// @Tags         admin/roles
// @Produce      json
// @Security     CookieAuth
// @Success      200 {array} accesscontrol.RoleResponse
// @Router       /admin/roles [get]
func (h *RoleHandler) List(c echo.Context) error {
	params := pagination.FromQuery(c.QueryParams())

	roles, total, err := h.roleRepo.ListWithPagination(
		c.Request().Context(),
		params.Page,
		params.Limit,
	)
	if err != nil {
		return apperror.Internal("failed to list roles").WithError(err)
	}

	// Load permissions for each role
	out := make([]accesscontrol.RoleResponse, 0, len(roles))
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
// @Success      200 {object} accesscontrol.RoleResponse
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
	if role.UserCount, err = h.roleRepo.CountUsers(c.Request().Context(), role.ID); err != nil {
		return apperror.Internal("failed to count users").WithError(err)
	}

	return c.JSON(http.StatusOK, role.ToResponse())
}

// Create godoc
// @Summary      Create a role
// @Description  Admin role and roles:write. Creates a role with its permissions and audits CREATE_ROLE (D-A21).
// @Tags         admin/roles
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        body body accesscontrol.CreateRoleRequest true "Role payload"
// @Success      201 {object} accesscontrol.RoleResponse
// @Failure      409 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /admin/roles [post]
func (h *RoleHandler) Create(c echo.Context) error {
	actor, err := actorID(c)
	if err != nil {
		return err
	}
	var req accesscontrol.CreateRoleRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	role, err := h.svc.Create(c.Request().Context(), actor, c.RealIP(), req)
	if err != nil {
		return roleError(err, "failed to create role")
	}
	return c.JSON(http.StatusCreated, role.ToResponse())
}

// Update godoc
// @Summary      Replace a role
// @Description  Admin role and roles:write. Replaces the name, description and whole permission set, and audits UPDATE_ROLE. Built-in roles (admin, editor, user) answer 409. A request that changes nothing answers 200 and audits nothing.
// @Tags         admin/roles
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "Role ID"
// @Param        body body accesscontrol.UpdateRoleRequest true "Role payload"
// @Success      200 {object} accesscontrol.RoleResponse
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /admin/roles/{id} [put]
func (h *RoleHandler) Update(c echo.Context) error {
	actor, err := actorID(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperror.BadRequest("invalid role id")
	}
	var req accesscontrol.UpdateRoleRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	role, err := h.svc.Update(c.Request().Context(), actor, c.RealIP(), id, req)
	if err != nil {
		return roleError(err, "failed to update role")
	}
	return c.JSON(http.StatusOK, role.ToResponse())
}

// Delete godoc
// @Summary      Delete a role
// @Description  Admin role and roles:write. Audits DELETE_ROLE. A built-in role, or one any account holds (deleted ones included), answers 409.
// @Tags         admin/roles
// @Security     CookieAuth
// @Param        id path int true "Role ID"
// @Success      204
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Router       /admin/roles/{id} [delete]
func (h *RoleHandler) Delete(c echo.Context) error {
	actor, err := actorID(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperror.BadRequest("invalid role id")
	}

	if err := h.svc.Delete(c.Request().Context(), actor, c.RealIP(), id); err != nil {
		return roleError(err, "failed to delete role")
	}
	return c.NoContent(http.StatusNoContent)
}
