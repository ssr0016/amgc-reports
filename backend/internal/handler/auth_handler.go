package handler

import (
	"errors"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
	sm          *scs.SessionManager
}

func NewAuthHandler(authService *service.AuthService, sm *scs.SessionManager) *AuthHandler {
	return &AuthHandler{authService: authService, sm: sm}
}

// messageAccountLocked is shown on login and on change-password for a locked account.
const messageAccountLocked = "Account locked. Try again later."

// Login godoc
// @Summary      Login user
// @Description  Authenticates user and creates a session
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body model.LoginRequest true "Login payload"
// @Success      200 {object} model.UserResponse
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c echo.Context) error {
	var req model.LoginRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	user, err := h.authService.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return apperror.Unauthorized("invalid email or password")
		}
		if errors.Is(err, service.ErrAccountLocked) {
			return apperror.Forbidden(messageAccountLocked)
		}
		switch {
		case errors.Is(err, service.ErrAccountInactive):
			return apperror.Forbidden("Your account is inactive. Contact your admin.")
		case errors.Is(err, service.ErrAccountUnderReview):
			return apperror.Forbidden("Your account is under review. Contact your admin.")
		case errors.Is(err, service.ErrAccountClosed):
			return apperror.Forbidden("This account is closed.")
		}
		return apperror.Internal("login failed").WithError(err)
	}

	ctx := c.Request().Context()
	if err := h.sm.RenewToken(ctx); err != nil {
		return apperror.Internal("session error").WithError(err)
	}
	h.sm.Put(ctx, "user_id", user.ID)

	return c.JSON(http.StatusOK, user.ToResponse())
}

// Logout godoc
// @Summary      Logout user
// @Description  Destroys the current session
// @Tags         auth
// @Produce      json
// @Security     CookieAuth
// @Success      200 {object} map[string]string
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c echo.Context) error {
	if err := h.sm.Destroy(c.Request().Context()); err != nil {
		return apperror.Internal("logout failed").WithError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "logged out"})
}

// ChangePassword godoc
// @Summary      Change own password
// @Description  Changes the signed-in user's password. The current password is checked against the same failure counter as login (5 wrong attempts lock the account for 15 minutes). The user's other sessions end; this one stays and its token is renewed. A wrong current password is a 422 on current_password, not a 401, so the client does not read it as a lost session.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        body body model.ChangePasswordRequest true "Current and new password"
// @Success      204
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Failure      429 {object} map[string]string
// @Router       /auth/change-password [post]
func (h *AuthHandler) ChangePassword(c echo.Context) error {
	ctx := c.Request().Context()
	userID := h.sm.GetInt64(ctx, "user_id")
	if userID == 0 {
		return apperror.Unauthorized("not authenticated")
	}
	var req model.ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	err := h.authService.ChangePassword(ctx, userID, c.RealIP(), h.sm.Token(ctx), req.CurrentPassword, req.NewPassword)
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		return fieldError("current_password", "is incorrect")
	case errors.Is(err, service.ErrPasswordUnchanged):
		return fieldError("new_password", "must differ from the current password")
	case errors.Is(err, service.ErrPasswordTooLong):
		return fieldError("new_password", "must be at most 72 bytes")
	case errors.Is(err, service.ErrAccountLocked):
		return apperror.Forbidden(messageAccountLocked)
	case errors.Is(err, service.ErrUnauthorized):
		return apperror.Unauthorized("not authenticated")
	case err != nil:
		return apperror.Internal("could not change the password").WithError(err)
	}

	// The password has changed and the other sessions are gone. A new token for this
	// one is extra safety, so a failure here must not report the change as failed.
	_ = h.sm.RenewToken(ctx)
	return c.NoContent(http.StatusNoContent)
}

func fieldError(field, message string) error {
	return apperror.Validation("validation failed").WithDetails(map[string]string{field: message})
}

// Me godoc
// @Summary      Get current user
// @Description  Returns the authenticated user's ID, name, email, role and permissions, read live from the database. Use it to show or hide actions; the server still enforces every permission.
// @Tags         auth
// @Produce      json
// @Security     CookieAuth
// @Success      200 {object} model.MeResponse
// @Failure      401 {object} map[string]string
// @Router       /auth/me [get]
func (h *AuthHandler) Me(c echo.Context) error {
	userID := h.sm.GetInt64(c.Request().Context(), "user_id")
	if userID == 0 {
		return apperror.Unauthorized("not authenticated")
	}
	me, err := h.authService.Me(c.Request().Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) {
			return apperror.Unauthorized("not authenticated")
		}
		return apperror.Internal("failed to load user").WithError(err)
	}
	return c.JSON(http.StatusOK, me)
}
