package apperror

import (
	"errors"
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

// ErrorHandler is the custom Echo HTTP error handler.
func ErrorHandler(err error, c echo.Context) {
	// If response already committed, skip
	if c.Response().Committed {
		return
	}

	appErr := resolve(err)

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

// resolve converts any error to the *AppError that ErrorHandler renders.
func resolve(err error) *AppError {
	if appErr, ok := As(err); ok {
		return appErr
	}
	var he *echo.HTTPError
	if errors.As(err, &he) {
		return fromHTTPError(he)
	}
	return Internal("internal server error").WithError(err)
}

// StatusOf returns the HTTP status ErrorHandler writes for err. Middleware
// that runs before ErrorHandler uses it to learn the final status.
func StatusOf(err error) int {
	return resolve(err).HTTPStatus
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
