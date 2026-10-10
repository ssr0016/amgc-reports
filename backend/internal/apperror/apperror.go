package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

// Code represents a machine-readable error code.
type Code string

const (
	CodeValidation   Code = "VALIDATION_ERROR"
	CodeNotFound     Code = "NOT_FOUND"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeConflict     Code = "CONFLICT"
	CodeRateLimit    Code = "RATE_LIMIT_EXCEEDED"
	CodeInternal     Code = "INTERNAL_ERROR"
	CodeBadRequest   Code = "BAD_REQUEST"
)

// AppError is a structured application error.
type AppError struct {
	Code       Code              `json:"code"`
	Message    string            `json:"message"`
	Details    map[string]string `json:"details,omitempty"`
	HTTPStatus int               `json:"-"`
	Err        error             `json:"-"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *AppError) Unwrap() error {
	return e.Err
}

// WithDetails adds details to the error.
func (e *AppError) WithDetails(details map[string]string) *AppError {
	e.Details = details
	return e
}

// WithError wraps an underlying error.
func (e *AppError) WithError(err error) *AppError {
	e.Err = err
	return e
}

// ============================================================
// Constructors
// ============================================================

func New(code Code, message string, status int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: status,
	}
}

func Validation(message string) *AppError {
	return New(CodeValidation, message, http.StatusUnprocessableEntity)
}

func BadRequest(message string) *AppError {
	return New(CodeBadRequest, message, http.StatusBadRequest)
}

func NotFound(message string) *AppError {
	return New(CodeNotFound, message, http.StatusNotFound)
}

func Unauthorized(message string) *AppError {
	return New(CodeUnauthorized, message, http.StatusUnauthorized)
}

func Forbidden(message string) *AppError {
	return New(CodeForbidden, message, http.StatusForbidden)
}

func Conflict(message string) *AppError {
	return New(CodeConflict, message, http.StatusConflict)
}

func RateLimit(message string) *AppError {
	return New(CodeRateLimit, message, http.StatusTooManyRequests)
}

func Internal(message string) *AppError {
	return New(CodeInternal, message, http.StatusInternalServerError)
}

// ============================================================
// Helpers
// ============================================================

// As converts a regular error to *AppError if possible.
func As(err error) (*AppError, bool) {
	var appErr *AppError
	ok := errors.As(err, &appErr)
	return appErr, ok
}

// Is checks if err matches the given error.
func Is(err, target error) bool {
	return errors.Is(err, target)
}
