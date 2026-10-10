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

type PermissionHandler struct {
	permissionRepo repository.PermissionRepository
}

func NewPermissionHandler(permissionRepo repository.PermissionRepository) *PermissionHandler {
	return &PermissionHandler{permissionRepo: permissionRepo}
}

// List godoc
// @Summary      List all permissions
// @Tags         admin/permissions
// @Produce      json
// @Security     CookieAuth
// @Param        resource query string false "Filter by resource"
// @Success      200 {array} model.PermissionResponse
// @Router       /admin/permissions [get]
func (h *PermissionHandler) List(c echo.Context) error {
	params := pagination.FromContext(c)
	resource := c.QueryParam("resource")

	perms, total, err := h.permissionRepo.ListWithPagination(
		c.Request().Context(),
		resource,
		params.Page,
		params.Limit,
	)
	if err != nil {
		return apperror.Internal("failed to list permissions").WithError(err)
	}

	out := make([]model.PermissionResponse, 0, len(perms))
	for i := range perms {
		out = append(out, perms[i].ToResponse())
	}

	return c.JSON(http.StatusOK, pagination.NewResponse(out, params, total))
}

// Get godoc
// @Summary      Get permission by ID
// @Tags         admin/permissions
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "Permission ID"
// @Success      200 {object} model.PermissionResponse
// @Router       /admin/permissions/{id} [get]
func (h *PermissionHandler) Get(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperror.BadRequest("invalid permission id")
	}

	perm, err := h.permissionRepo.GetByID(c.Request().Context(), id)
	if err != nil {
		return apperror.Internal("failed to get permission").WithError(err)
	}
	if perm == nil {
		return apperror.NotFound("permission not found")
	}
	return c.JSON(http.StatusOK, perm.ToResponse())
}

// Create godoc
// @Summary      Create a new permission
// @Tags         admin/permissions
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        body body CreatePermissionRequest true "Permission payload"
// @Success      201 {object} model.PermissionResponse
// @Router       /admin/permissions [post]
func (h *PermissionHandler) Create(c echo.Context) error {
	var req CreatePermissionRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return apperror.Validation(err.Error())
	}

	existing, err := h.permissionRepo.GetByName(c.Request().Context(), req.Name)
	if err != nil {
		return apperror.Internal("failed to check permission").WithError(err)
	}
	if existing != nil {
		return apperror.Conflict("permission name already exists")
	}

	perm, err := h.permissionRepo.Create(c.Request().Context(), req.Name, req.Resource, req.Action)
	if err != nil {
		return apperror.Internal("failed to create permission").WithError(err)
	}
	return c.JSON(http.StatusCreated, perm.ToResponse())
}

// Delete godoc
// @Summary      Delete a permission
// @Tags         admin/permissions
// @Security     CookieAuth
// @Param        id path int true "Permission ID"
// @Success      204
// @Router       /admin/permissions/{id} [delete]
func (h *PermissionHandler) Delete(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return apperror.BadRequest("invalid permission id")
	}

	if err := h.permissionRepo.Delete(c.Request().Context(), id); err != nil {
		return apperror.Internal("failed to delete permission").WithError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// CreatePermissionRequest is the payload for creating a permission.
type CreatePermissionRequest struct {
	Name     string `json:"name" validate:"required,min=3,max=100"`
	Resource string `json:"resource" validate:"required,min=2,max=50"`
	Action   string `json:"action" validate:"required,min=2,max=50"`
}
