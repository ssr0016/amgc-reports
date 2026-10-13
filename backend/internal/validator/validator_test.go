package validator_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

// validationError runs Validate and requires it to fail with an *AppError.
func validationError(t *testing.T, v any) *apperror.AppError {
	t.Helper()
	err := validator.New().Validate(v)
	if err == nil {
		t.Fatal("Validate() = nil, want a validation error")
	}
	appErr, ok := apperror.As(err)
	if !ok {
		t.Fatalf("Validate() error = %T (%v), want *apperror.AppError", err, err)
	}
	return appErr
}

func TestValidate_ValidInput_ReturnsNil(t *testing.T) {
	type req struct {
		Email string `json:"email" validate:"required,email"`
	}
	if err := validator.New().Validate(&req{Email: "a@example.com"}); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestValidate_Invalid_ReturnsAppErrorWithFieldDetails(t *testing.T) {
	type req struct {
		Email string `json:"email" validate:"required,email"`
		Name  string `json:"name" validate:"required"`
	}
	appErr := validationError(t, &req{})

	if appErr.Code != apperror.CodeValidation {
		t.Errorf("Code = %q, want %q", appErr.Code, apperror.CodeValidation)
	}
	if appErr.HTTPStatus != http.StatusUnprocessableEntity {
		t.Errorf("HTTPStatus = %d, want %d", appErr.HTTPStatus, http.StatusUnprocessableEntity)
	}
	if appErr.Message != "validation failed" {
		t.Errorf("Message = %q, want %q", appErr.Message, "validation failed")
	}
	want := map[string]string{"email": "is required", "name": "is required"}
	assertDetails(t, appErr.Details, want)
}

func TestValidate_UsesJSONFieldNames(t *testing.T) {
	type req struct {
		NewPassword string `json:"new_password" validate:"required"`
		Skipped     string `json:"-" validate:"required"`
		NoTag       string `validate:"required"`
	}
	appErr := validationError(t, &req{})

	if _, ok := appErr.Details["new_password"]; !ok {
		t.Errorf("Details = %v, want key new_password (the json name)", appErr.Details)
	}
	if _, ok := appErr.Details["NewPassword"]; ok {
		t.Errorf("Details = %v, must not use the Go field name", appErr.Details)
	}
}

func TestValidate_MessagePerTag(t *testing.T) {
	type strMin struct {
		V string `json:"v" validate:"min=8"`
	}
	type strMax struct {
		V string `json:"v" validate:"max=3"`
	}
	type numMin struct {
		V int64 `json:"v" validate:"min=5"`
	}
	type numMax struct {
		V int64 `json:"v" validate:"max=5"`
	}
	type required struct {
		V string `json:"v" validate:"required"`
	}
	type email struct {
		V string `json:"v" validate:"email"`
	}
	type strong struct {
		V string `json:"v" validate:"strongpass"`
	}
	type other struct {
		V string `json:"v" validate:"oneof=a b"`
	}

	tests := []struct {
		name string
		in   any
		want string
	}{
		{"required", &required{}, "is required"},
		{"email", &email{V: "not-an-email"}, "must be a valid email address"},
		{"min string", &strMin{V: "abc"}, "must be at least 8 characters"},
		{"max string", &strMax{V: "abcdef"}, "must be at most 3 characters"},
		{"min numeric", &numMin{V: 1}, "must be at least 5"},
		{"max numeric", &numMax{V: 9}, "must be at most 5"},
		{"strongpass", &strong{V: "aaaa"}, "is too weak; use a longer, more varied password"},
		{"unmapped tag", &other{V: "z"}, "invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appErr := validationError(t, tt.in)
			if got := appErr.Details["v"]; got != tt.want {
				t.Errorf("Details[v] = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidate_OneMessagePerField(t *testing.T) {
	type req struct {
		// Fails both min and email; the first failing tag wins.
		Email string `json:"email" validate:"min=10,email"`
	}
	appErr := validationError(t, &req{Email: "abc"})

	if len(appErr.Details) != 1 {
		t.Fatalf("Details = %v, want exactly one entry", appErr.Details)
	}
	if got, want := appErr.Details["email"], "must be at least 10 characters"; got != want {
		t.Errorf("Details[email] = %q, want %q", got, want)
	}
}

func TestValidate_DoesNotEchoSubmittedValue(t *testing.T) {
	const secret = "hunter2-correct-horse"
	type req struct {
		Password string `json:"password" validate:"min=100"`
	}
	appErr := validationError(t, &req{Password: secret})

	body, err := json.Marshal(appErr)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), secret) {
		t.Errorf("serialized error %s contains the submitted value", body)
	}
}

func TestValidate_NonStructInput_FallsBackTo422WithoutDetails(t *testing.T) {
	appErr := validationError(t, "not a struct")

	if appErr.Code != apperror.CodeValidation {
		t.Errorf("Code = %q, want %q", appErr.Code, apperror.CodeValidation)
	}
	if appErr.HTTPStatus != http.StatusUnprocessableEntity {
		t.Errorf("HTTPStatus = %d, want %d", appErr.HTTPStatus, http.StatusUnprocessableEntity)
	}
	if len(appErr.Details) != 0 {
		t.Errorf("Details = %v, want none", appErr.Details)
	}
}

func assertDetails(t *testing.T, got, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("Details = %v, want %v", got, want)
		return
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("Details[%s] = %q, want %q", k, got[k], w)
		}
	}
}
