package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
	cookies     *Cookies
}

func NewAuthHandler(authService *service.AuthService, cookies *Cookies) *AuthHandler {
	return &AuthHandler{authService: authService, cookies: cookies}
}

// messageAccountLocked is shown on login and on change-password for a locked account.
const messageAccountLocked = "Account locked. Try again later."

// Login godoc
// @Summary      Login user
// @Description  Authenticates the user and sets the access_token and refresh_token cookies (HttpOnly)
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

	pair, err := h.authService.StartSession(c.Request().Context(), user.ID, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		return apperror.Internal("session error").WithError(err)
	}
	h.cookies.setPair(c, pair)

	return c.JSON(http.StatusOK, user.ToResponse())
}

// Logout godoc
// @Summary      Logout user
// @Description  Ends the sign-in the refresh_token cookie belongs to and clears both cookies. It needs no valid access token, so it works after that has expired.
// @Tags         auth
// @Produce      json
// @Success      200 {object} map[string]string
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c echo.Context) error {
	h.cookies.clear(c) // before the body: cookies cannot be set once it is written
	if raw := refreshCookie(c); raw != "" {
		if err := h.authService.EndSession(c.Request().Context(), raw); err != nil {
			return apperror.Internal("logout failed").WithError(err)
		}
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "logged out"})
}

// Refresh godoc
// @Summary      Refresh the sign-in
// @Description  Trades the refresh_token cookie for a new access token and a new refresh token, both set as cookies. The old refresh token stops working. Answers 401 and clears the cookies when the refresh token is unknown, expired, idle too long, revoked, or its account may no longer sign in. A token that was rotated within the last 10 seconds (a second tab) is a 401 that leaves the cookies alone.
// @Tags         auth
// @Success      204
// @Failure      401 {object} map[string]string
// @Failure      429 {object} map[string]string
// @Router       /auth/refresh [post]
func (h *AuthHandler) Refresh(c echo.Context) error {
	raw := refreshCookie(c)
	if raw == "" {
		return apperror.Unauthorized("authentication required")
	}
	pair, err := h.authService.Refresh(c.Request().Context(), raw, c.RealIP(), c.Request().UserAgent())
	switch {
	case errors.Is(err, service.ErrRefreshReplayed):
		// Another tab just refreshed with this token. The cookies now hold its new
		// tokens (the jar is shared), so clearing them would end a live sign-in.
		return apperror.Unauthorized("authentication required")
	case errors.Is(err, service.ErrInvalidRefresh), errors.Is(err, service.ErrUnauthorized):
		h.cookies.clear(c)
		return apperror.Unauthorized("authentication required")
	case err != nil:
		return apperror.Internal("could not refresh the session").WithError(err)
	}
	h.cookies.setPair(c, pair)
	return c.NoContent(http.StatusNoContent)
}

// ChangePassword godoc
// @Summary      Change own password
// @Description  Changes the signed-in user's password. The current password is checked against the same failure counter as login (5 wrong attempts lock the account for 15 minutes). The user's other sign-ins end; this one stays and its access token is renewed. A wrong current password is a 422 on current_password, not a 401, so the client does not read it as a lost session. confirm_password is required and must equal new_password exactly (422 on confirm_password); that check runs first, so a typo does not count as a failed attempt.
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
	caller, ok := request.UserFrom(ctx)
	if !ok {
		return apperror.Unauthorized("not authenticated")
	}
	userID := caller.ID
	var req model.ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	err := h.authService.ChangePassword(ctx, userID, c.RealIP(), caller.SessionID, req.CurrentPassword, req.NewPassword)
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

	// The password change moved tokens_valid_after past this sign-in's access token,
	// so it needs a new one. A failure here must not report the change as failed:
	// the next request refreshes.
	if access, exp, err := h.authService.RenewAccess(caller.ID, caller.SessionID); err == nil {
		h.cookies.SetAccess(c, access, exp)
	}
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
	caller, ok := request.UserFrom(c.Request().Context())
	if !ok {
		return apperror.Unauthorized("not authenticated")
	}
	me, err := h.authService.Me(c.Request().Context(), caller.ID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) {
			return apperror.Unauthorized("not authenticated")
		}
		return apperror.Internal("failed to load user").WithError(err)
	}
	return c.JSON(http.StatusOK, me)
}
