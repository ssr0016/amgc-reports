package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
)

// UserAdminService is what AdminUserHandler needs to create and edit accounts,
// so tests can pass a fake. main passes *service.UserService.
type UserAdminService interface {
	Create(ctx context.Context, actorID int64, ip string, req model.CreateUserRequest) (*model.User, error)
	Update(ctx context.Context, actorID int64, ip string, id int64, req model.UpdateUserRequest) (*model.User, error)
	ChangeStatus(ctx context.Context, actorID int64, ip string, id int64, status string) (*model.User, error)
}

var _ UserAdminService = (*service.UserService)(nil)

type AdminUserHandler struct {
	userRepo repository.UserRepository
	roleRepo repository.RoleRepository
	svc      UserAdminService
	sm       *scs.SessionManager
}

func NewAdminUserHandler(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	svc UserAdminService,
	sm *scs.SessionManager,
) *AdminUserHandler {
	return &AdminUserHandler{
		userRepo: userRepo,
		roleRepo: roleRepo,
		svc:      svc,
		sm:       sm,
	}
}

// userError maps service errors to API errors; anything unexpected is a 500
// that carries no detail.
func userError(err error, msg string) error {
	switch {
	case errors.Is(err, service.ErrEmailTaken):
		return apperror.Conflict(err.Error())
	case errors.Is(err, service.ErrRoleNotFound):
		return apperror.Validation("role not found").WithDetails(map[string]string{"role_id": "role not found"})
	case errors.Is(err, service.ErrUserNotFound):
		return apperror.NotFound("user not found")
	case errors.Is(err, service.ErrUnauthorized):
		return apperror.Unauthorized("authentication required")
	case errors.Is(err, service.ErrCannotChangeOwnStatus):
		return apperror.Forbidden("You cannot change your own status.")
	case errors.Is(err, service.ErrLastActiveAdmin):
		return apperror.Conflict("At least one active admin is required.")
	case errors.Is(err, service.ErrInvalidUserStatus):
		return apperror.Validation("invalid status").WithDetails(map[string]string{"status": "invalid status"})
	}
	return apperror.Internal(msg).WithError(err)
}

func (h *AdminUserHandler) actorID(c echo.Context) (int64, error) {
	id := h.sm.GetInt64(c.Request().Context(), "user_id")
	if id == 0 {
		return 0, apperror.Unauthorized("authentication required")
	}
	return id, nil
}

// CreateUser godoc
// @Summary      Create a user
// @Description  Admin only. Creates an account with an optional role (default: user) and the profile text that prefills the report form. This is the only way to create an account.
// @Tags         admin/users
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        body body model.CreateUserRequest true "New account"
// @Success      201 {object} model.UserResponse
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /admin/users [post]
func (h *AdminUserHandler) CreateUser(c echo.Context) error {
	actor, err := h.actorID(c)
	if err != nil {
		return err
	}
	var req model.CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}
	user, err := h.svc.Create(c.Request().Context(), actor, c.RealIP(), req)
	if err != nil {
		return userError(err, "failed to create user")
	}
	return c.JSON(http.StatusCreated, user.ToResponse())
}

// UpdateUser godoc
// @Summary      Update a user's name and profile
// @Description  Admin only. Replaces the name and the area and church text. The password and role are not changed here.
// @Tags         admin/users
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "User ID"
// @Param        body body model.UpdateUserRequest true "Name and profile"
// @Success      200 {object} model.UserResponse
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /admin/users/{id} [put]
func (h *AdminUserHandler) UpdateUser(c echo.Context) error {
	actor, err := h.actorID(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return apperror.BadRequest("invalid user id")
	}
	var req model.UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}
	user, err := h.svc.Update(c.Request().Context(), actor, c.RealIP(), id, req)
	if err != nil {
		return userError(err, "failed to update user")
	}
	return c.JSON(http.StatusOK, user.ToResponse())
}

// ChangeStatus godoc
// @Summary      Change a user's status
// @Description  Admin only. Sets the status to active, inactive, disciplinary or deceased. Only an active account can sign in; moving a user away from active ends all of their sessions. You cannot change your own status, and the only active admin cannot be moved away from active. The same status as now answers 200 and changes and audits nothing.
// @Tags         admin/users
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "User ID"
// @Param        body body model.ChangeStatusRequest true "New status"
// @Success      200 {object} model.UserResponse
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /admin/users/{id}/status [put]
func (h *AdminUserHandler) ChangeStatus(c echo.Context) error {
	actor, err := h.actorID(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return apperror.BadRequest("invalid user id")
	}
	var req model.ChangeStatusRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}
	user, err := h.svc.ChangeStatus(c.Request().Context(), actor, c.RealIP(), id, req.Status)
	if err != nil {
		return userError(err, "failed to change user status")
	}
	return c.JSON(http.StatusOK, user.ToResponse())
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
		return err
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
		// Either row can be deleted between the checks above and this update:
		// the role trips the foreign key, the user matches no row.
		if errors.Is(err, repository.ErrForeignKeyViolation) {
			return apperror.NotFound("role not found")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return apperror.NotFound("user not found")
		}
		return apperror.Internal("failed to update role").WithError(err)
	}

	// Reload user with role
	updatedUser, err := h.userRepo.GetWithRole(c.Request().Context(), id)
	if err != nil {
		return apperror.Internal("failed to reload user").WithError(err)
	}

	return c.JSON(http.StatusOK, updatedUser.ToResponse())
}
