package rest

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
)

type PasswordResetHandler struct {
	service auth.PasswordResetService
}

func NewPasswordResetHandler(svc auth.PasswordResetService) *PasswordResetHandler {
	return &PasswordResetHandler{service: svc}
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
} // @name internal_handler.ForgotPasswordRequest

type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,strongpass"`
	// ConfirmPassword must equal NewPassword exactly; checked before the token is looked up.
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=NewPassword"`
} // @name internal_handler.ResetPasswordRequest

// ForgotPassword godoc
// @Summary      Request password reset
// @Description  Sends a password reset token to the email if it exists
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body ForgotPasswordRequest true "Email payload"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Router       /auth/forgot-password [post]
func (h *PasswordResetHandler) ForgotPassword(c echo.Context) error {
	var req ForgotPasswordRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	if err := h.service.RequestReset(c.Request().Context(), req.Email); err != nil {
		return apperror.Internal("failed to process request").WithError(err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "if the email exists, a reset link has been sent",
	})
}

// ResetPassword godoc
// @Summary      Reset password with token
// @Description  Resets the user's password using a valid reset token. The new password follows the same rules as change-password: strong enough (422 on new_password) and at most 72 bytes. confirm_password is required and must equal new_password exactly (422 on confirm_password), checked before the token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body ResetPasswordRequest true "Reset payload"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /auth/reset-password [post]
func (h *PasswordResetHandler) ResetPassword(c echo.Context) error {
	var req ResetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	if err := h.service.ResetPassword(c.Request().Context(), req.Token, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidResetToken):
			return apperror.BadRequest("invalid or expired token")
		case errors.Is(err, auth.ErrPasswordTooLong):
			return fieldError("new_password", "must be at most 72 bytes")
		}
		return apperror.Internal("password reset failed").WithError(err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "password reset successful",
	})
}
