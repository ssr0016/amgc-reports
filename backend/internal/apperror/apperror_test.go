package apperror

import (
	"errors"
	"net/http"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(CodeValidation, "test message", http.StatusUnprocessableEntity)

	if err.Code != CodeValidation {
		t.Errorf("Code = %s, want %s", err.Code, CodeValidation)
	}
	if err.Message != "test message" {
		t.Errorf("Message = %s, want 'test message'", err.Message)
	}
	if err.HTTPStatus != http.StatusUnprocessableEntity {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusUnprocessableEntity)
	}
}

func TestConstructors(t *testing.T) {
	tests := []struct {
		name       string
		err        *AppError
		wantCode   Code
		wantStatus int
	}{
		{"Validation", Validation("msg"), CodeValidation, http.StatusUnprocessableEntity},
		{"BadRequest", BadRequest("msg"), CodeBadRequest, http.StatusBadRequest},
		{"NotFound", NotFound("msg"), CodeNotFound, http.StatusNotFound},
		{"Unauthorized", Unauthorized("msg"), CodeUnauthorized, http.StatusUnauthorized},
		{"Forbidden", Forbidden("msg"), CodeForbidden, http.StatusForbidden},
		{"Conflict", Conflict("msg"), CodeConflict, http.StatusConflict},
		{"RateLimit", RateLimit("msg"), CodeRateLimit, http.StatusTooManyRequests},
		{"Internal", Internal("msg"), CodeInternal, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCode {
				t.Errorf("Code = %s, want %s", tt.err.Code, tt.wantCode)
			}
			if tt.err.HTTPStatus != tt.wantStatus {
				t.Errorf("HTTPStatus = %d, want %d", tt.err.HTTPStatus, tt.wantStatus)
			}
		})
	}
}

func TestWithDetails(t *testing.T) {
	err := Validation("invalid").WithDetails(map[string]string{
		"email": "required",
		"name":  "too short",
	})

	if err.Details["email"] != "required" {
		t.Errorf("Details[email] = %s, want required", err.Details["email"])
	}
	if err.Details["name"] != "too short" {
		t.Errorf("Details[name] = %s, want 'too short'", err.Details["name"])
	}
}

func TestWithError(t *testing.T) {
	inner := errors.New("inner error")
	err := Internal("something failed").WithError(inner)

	if err.Err != inner {
		t.Errorf("Err = %v, want %v", err.Err, inner)
	}
}

func TestError(t *testing.T) {
	err := NotFound("user not found")
	expected := "NOT_FOUND: user not found"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}

	// With inner error
	errWithInner := Internal("failed").WithError(errors.New("db error"))
	if !contains(errWithInner.Error(), "db error") {
		t.Errorf("Error() = %s, want to contain 'db error'", errWithInner.Error())
	}
}

func TestAs(t *testing.T) {
	appErr := NotFound("test")

	// Test with direct AppError
	got, ok := As(appErr)
	if !ok {
		t.Fatal("As() returned false for AppError")
	}
	if got.Code != CodeNotFound {
		t.Errorf("Code = %s, want %s", got.Code, CodeNotFound)
	}

	// Test with wrapped error
	wrapped := errors.New("wrapper: " + appErr.Error())
	_, ok = As(wrapped)
	if ok {
		t.Error("As() returned true for non-AppError")
	}

	// Test with errors.As chain
	inner := NotFound("inner")
	outer := Internal("outer").WithError(inner)
	got, ok = As(outer)
	if !ok {
		t.Fatal("As() returned false for AppError chain")
	}
	if got.Code != CodeInternal {
		t.Errorf("Code = %s, want %s", got.Code, CodeInternal)
	}
}

func TestUnwrap(t *testing.T) {
	inner := errors.New("inner")
	err := Internal("outer").WithError(inner)

	if err.Unwrap() != inner {
		t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), inner)
	}
}

func TestIs(t *testing.T) {
	err1 := NotFound("test")
	err2 := NotFound("test")

	if Is(err1, err2) {
		t.Error("Is() = true for different error instances")
	}

	if !Is(err1, err1) {
		t.Error("Is() = false for same error instance")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
