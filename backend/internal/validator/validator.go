package validator

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	passwordvalidator "github.com/wagslane/go-password-validator"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
)

type CustomValidator struct {
	validator *validator.Validate
}

func New() *CustomValidator {
	v := &CustomValidator{validator: validator.New()}
	v.validator.RegisterTagNameFunc(jsonName)
	_ = v.validator.RegisterValidation("strongpass", validateStrongPassword)
	return v
}

// Validate returns nil, or an *apperror.AppError (422) whose Details map each
// failing field's JSON name to one message. It never includes submitted values.
func (cv *CustomValidator) Validate(i interface{}) error {
	err := cv.validator.Struct(i)
	if err == nil {
		return nil
	}

	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return apperror.Validation(err.Error())
	}

	details := make(map[string]string, len(verrs))
	for _, fe := range verrs {
		if _, seen := details[fe.Field()]; !seen {
			details[fe.Field()] = message(fe)
		}
	}
	return apperror.Validation("validation failed").WithDetails(details)
}

// jsonName makes FieldError.Field() the JSON key so details match the request
// body. An empty result keeps the Go field name.
func jsonName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	if name == "-" {
		return ""
	}
	return name
}

func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "strongpass":
		return "is too weak; use a longer, more varied password"
	case "min":
		return bound("at least", fe)
	case "max":
		return bound("at most", fe)
	}
	return "invalid"
}

// bound words min/max. shortcut: only strings get "characters"; slices and maps
// get the bare number, add "items" when a validated DTO carries one.
func bound(limit string, fe validator.FieldError) string {
	if fe.Kind() == reflect.String {
		return fmt.Sprintf("must be %s %s characters", limit, fe.Param())
	}
	return fmt.Sprintf("must be %s %s", limit, fe.Param())
}

func validateStrongPassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	err := passwordvalidator.Validate(password, 60)
	return err == nil
}

var _ echo.Validator = (*CustomValidator)(nil)
