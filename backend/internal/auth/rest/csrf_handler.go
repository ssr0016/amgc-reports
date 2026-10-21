package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	ourmiddleware "github.com/ssr0016/pastoral-reports/backend/internal/middleware"
)

// CSRFTokenResponse is returned by the CSRF token endpoint.
type CSRFTokenResponse struct {
	CSRFToken string `json:"csrf_token"`
} // @name internal_handler.CSRFTokenResponse

// CSRFToken godoc
// @Summary      Get CSRF token
// @Description  Returns the CSRF token for the current client. The same value is set in the HttpOnly csrf_token cookie; send it back in the X-CSRF-Token header on POST/PUT/DELETE requests.
// @Tags         auth
// @Produce      json
// @Success      200 {object} CSRFTokenResponse
// @Router       /auth/csrf [get]
func CSRFToken(c echo.Context) error {
	return c.JSON(http.StatusOK, CSRFTokenResponse{CSRFToken: ourmiddleware.GetCSRFToken(c)})
}
