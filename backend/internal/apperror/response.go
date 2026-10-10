package apperror

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
)

// ErrorResponse is the standard JSON error payload.
type ErrorResponse struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// WriteError writes an error response to the Echo context.
func WriteError(c echo.Context, err error) error {
	appErr, ok := As(err)
	if !ok {
		// Unknown error → internal server error
		appErr = Internal("internal server error").WithError(err)
	}

	// Hide internal error messages in production
	if os.Getenv("APP_ENV") == "production" && appErr.HTTPStatus >= 500 {
		appErr.Message = "internal server error"
	}

	resp := ErrorResponse{
		Error:   string(appErr.Code),
		Message: appErr.Message,
		Details: appErr.Details,
	}

	return c.JSON(appErr.HTTPStatus, resp)
}

// ErrorHandler is the custom Echo HTTP error handler.
func ErrorHandler(err error, c echo.Context) {
	// If response already committed, skip
	if c.Response().Committed {
		return
	}

	// Try to convert to AppError
	appErr, ok := As(err)
	if !ok {
		// Handle Echo's HTTPError
		if he, isHTTP := err.(*echo.HTTPError); isHTTP {
			appErr = fromHTTPError(he)
		} else {
			appErr = Internal("internal server error").WithError(err)
		}
	}

	// Hide internal error messages in production
	if os.Getenv("APP_ENV") == "production" && appErr.HTTPStatus >= 500 {
		appErr.Message = "internal server error"
	}

	resp := ErrorResponse{
		Error:   string(appErr.Code),
		Message: appErr.Message,
		Details: appErr.Details,
	}

	if writeErr := c.JSON(appErr.HTTPStatus, resp); writeErr != nil {
		c.Logger().Error(writeErr)
	}
}

// fromHTTPError converts an Echo HTTPError to AppError.
func fromHTTPError(he *echo.HTTPError) *AppError {
	msg, ok := he.Message.(string)
	if !ok {
		msg = http.StatusText(he.Code)
	}

	var code Code
	switch he.Code {
	case http.StatusBadRequest:
		code = CodeBadRequest
	case http.StatusUnauthorized:
		code = CodeUnauthorized
	case http.StatusForbidden:
		code = CodeForbidden
	case http.StatusNotFound:
		code = CodeNotFound
	case http.StatusConflict:
		code = CodeConflict
	case http.StatusTooManyRequests:
		code = CodeRateLimit
	default:
		if he.Code >= 500 {
			code = CodeInternal
		} else {
			code = CodeBadRequest
		}
	}

	return &AppError{
		Code:       code,
		Message:    msg,
		HTTPStatus: he.Code,
	}
}
