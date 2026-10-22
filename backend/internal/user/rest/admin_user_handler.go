package rest

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

type AdminUserHandler struct {
	userRepo user.Store
	svc      user.Service
}

func NewAdminUserHandler(userRepo user.Store, svc user.Service) *AdminUserHandler {
	return &AdminUserHandler{userRepo: userRepo, svc: svc}
}

// userError maps service errors to API errors; anything unexpected is a 500
// that carries no detail.
func userError(err error, msg string) error {
	switch {
	case errors.Is(err, user.ErrEmailTaken):
		return apperror.Conflict(err.Error())
	case errors.Is(err, user.ErrRoleNotFound):
		return apperror.Validation("role not found").WithDetails(map[string]string{"role_id": "role not found"})
	case errors.Is(err, user.ErrUserNotFound):
		return apperror.NotFound("user not found")
	case errors.Is(err, auth.ErrUnauthorized):
		return apperror.Unauthorized("authentication required")
	case errors.Is(err, user.ErrCannotChangeOwnStatus):
		return apperror.Forbidden("You cannot change your own status.")
	case errors.Is(err, user.ErrCannotResetOwnPassword):
		return apperror.Forbidden("You cannot reset your own password. Use the Change Password page instead.")
	case errors.Is(err, user.ErrLastActiveAdmin):
		return apperror.Conflict("At least one active admin is required.")
	case errors.Is(err, user.ErrCannotChangeOwnRole):
		return apperror.Forbidden("You cannot change your own role.")
	case errors.Is(err, user.ErrCannotDeleteSelf):
		return apperror.Forbidden("You cannot delete your own account.")
	case errors.Is(err, user.ErrEmailBelongsToDeletedUser):
		return apperror.Conflict("This email belongs to a deleted user. Restore them instead.")
	case errors.Is(err, user.ErrInvalidUserStatus):
		return apperror.Validation("invalid status").WithDetails(map[string]string{"status": "invalid status"})
	}
	return apperror.Internal(msg).WithError(err)
}

func (h *AdminUserHandler) actorID(c echo.Context) (int64, error) {
	u, ok := request.UserFrom(c.Request().Context())
	if !ok {
		return 0, apperror.Unauthorized("authentication required")
	}
	return u.ID, nil
}

// CreateUser godoc
// @Summary      Create a user
// @Description  Admin only. Creates an account with an optional role (default: user) and the profile text that prefills the report form. This is the only way to create an account. confirm_password is required and must equal password exactly (422 on confirm_password).
// @Tags         admin/users
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        body body user.CreateUserRequest true "New account"
// @Success      201 {object} user.UserResponse
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
	var req user.CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}
	usr, err := h.svc.Create(c.Request().Context(), actor, c.RealIP(), req)
	if err != nil {
		return userError(err, "failed to create user")
	}
	return c.JSON(http.StatusCreated, usr.ToResponse())
}

// UpdateUser godoc
// @Summary      Update a user's name and profile
// @Description  Admin only. Replaces the name and the area and church text. The password and role are not changed here.
// @Tags         admin/users
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "User ID"
// @Param        body body user.UpdateUserRequest true "Name and profile"
// @Success      200 {object} user.UserResponse
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
	var req user.UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}
	usr, err := h.svc.Update(c.Request().Context(), actor, c.RealIP(), id, req)
	if err != nil {
		return userError(err, "failed to update user")
	}
	return c.JSON(http.StatusOK, usr.ToResponse())
}

// ChangeStatus godoc
// @Summary      Change a user's status
// @Description  Admin only. Sets the status to active, inactive, disciplinary or deceased. Only an active account can sign in; moving a user away from active ends all of their sessions. You cannot change your own status, and the only active admin cannot be moved away from active. The same status as now answers 200 and changes and audits nothing.
// @Tags         admin/users
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "User ID"
// @Param        body body user.ChangeStatusRequest true "New status"
// @Success      200 {object} user.UserResponse
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
	var req user.ChangeStatusRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}
	usr, err := h.svc.ChangeStatus(c.Request().Context(), actor, c.RealIP(), id, req.Status)
	if err != nil {
		return userError(err, "failed to change user status")
	}
	return c.JSON(http.StatusOK, usr.ToResponse())
}

// DeleteUser godoc
// @Summary      Soft-delete a user
// @Description  Admin only. Marks the account deleted with an optional reason (at most 500 characters, trimmed). A deleted user cannot sign in, gets the same answer as an unknown email, and all of their sessions end. Their email stays reserved and their status does not change. You cannot delete your own account, and the only active admin cannot be deleted. An already-deleted user answers 200 and changes and audits nothing. The body may be empty.
// @Tags         admin/users
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "User ID"
// @Param        body body user.DeleteUserRequest false "Optional reason"
// @Success      200 {object} user.UserResponse
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /admin/users/{id} [delete]
func (h *AdminUserHandler) DeleteUser(c echo.Context) error {
	actor, err := h.actorID(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return apperror.BadRequest("invalid user id")
	}
	var req user.DeleteUserRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}
	usr, err := h.svc.Delete(c.Request().Context(), actor, c.RealIP(), id, req.Reason)
	if err != nil {
		return userError(err, "failed to delete user")
	}
	return c.JSON(http.StatusOK, usr.ToResponse())
}

// RestoreUser godoc
// @Summary      Restore a deleted user
// @Description  Admin only. Clears the deleted mark, so the user can sign in again if their status is active; the status does not change. The audit row keeps who deleted them, when and why. A user who is not deleted answers 200 and changes and audits nothing.
// @Tags         admin/users
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "User ID"
// @Success      200 {object} user.UserResponse
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /admin/users/{id}/restore [post]
func (h *AdminUserHandler) RestoreUser(c echo.Context) error {
	actor, err := h.actorID(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return apperror.BadRequest("invalid user id")
	}
	usr, err := h.svc.Restore(c.Request().Context(), actor, c.RealIP(), id)
	if err != nil {
		return userError(err, "failed to restore user")
	}
	return c.JSON(http.StatusOK, usr.ToResponse())
}

// ResetPassword godoc
// @Summary      Reset a user's password
// @Description  Admin only. Sets a new password for another user (they never see the old one, and nobody can read it back). It ends all of the user's sessions, clears their login lockout and is audited as ADMIN_RESET_PASSWORD. The new password follows the change-password rules: strong enough (422 on new_password), at most 72 bytes, and confirm_password must equal it exactly (422 on confirm_password). You cannot reset your own password (403): use change-password. Any status and any role may be reset; the status does not change.
// @Tags         admin/users
// @Accept       json
// @Security     CookieAuth
// @Param        id path int true "User ID"
// @Param        body body user.AdminResetPasswordRequest true "New password and its confirmation"
// @Success      204
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Failure      429 {object} map[string]string
// @Router       /admin/users/{id}/reset-password [post]
func (h *AdminUserHandler) ResetPassword(c echo.Context) error {
	actor, err := h.actorID(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return apperror.BadRequest("invalid user id")
	}
	var req user.AdminResetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}
	err = h.svc.ResetPassword(c.Request().Context(), actor, c.RealIP(), id, req.NewPassword)
	if errors.Is(err, auth.ErrPasswordTooLong) {
		return fieldError("new_password", "must be at most 72 bytes")
	}
	if err != nil {
		return userError(err, "failed to reset password")
	}
	return c.NoContent(http.StatusNoContent)
}

// GetUser godoc
// @Summary      Get user with role
// @Tags         admin/users
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "User ID"
// @Success      200 {object} user.UserResponse
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
// @Description  Admin role and users:write. Gives the user another role and audits CHANGE_USER_ROLE (D-A21). You cannot change your own role (403), and the only active admin cannot be moved off admin (409). The same role as now answers 200 and audits nothing. A deleted account may be changed.
// @Tags         admin/users
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "User ID"
// @Param        body body accesscontrol.ChangeRoleRequest true "Role change payload"
// @Success      200 {object} user.UserResponse
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Router       /admin/users/{id}/role [put]
func (h *AdminUserHandler) ChangeRole(c echo.Context) error {
	actor, err := h.actorID(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return apperror.BadRequest("invalid user id")
	}
	var req accesscontrol.ChangeRoleRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	usr, err := h.svc.ChangeRole(c.Request().Context(), actor, c.RealIP(), id, req.RoleID)
	if errors.Is(err, user.ErrRoleNotFound) {
		return apperror.NotFound("role not found") // the role is what this request names, unlike Create's optional field
	}
	if err != nil {
		return userError(err, "failed to change role")
	}
	return c.JSON(http.StatusOK, usr.ToResponse())
}

func fieldError(field, message string) error {
	return apperror.Validation("validation failed").WithDetails(map[string]string{field: message})
}
