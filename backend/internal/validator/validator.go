package validator

import (
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	passwordvalidator "github.com/wagslane/go-password-validator"
)

type CustomValidator struct {
	validator *validator.Validate
}

func New() *CustomValidator {
	v := &CustomValidator{validator: validator.New()}
	_ = v.validator.RegisterValidation("strongpass", validateStrongPassword)
	return v
}

func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

func validateStrongPassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	err := passwordvalidator.Validate(password, 60)
	return err == nil
}

var _ echo.Validator = (*CustomValidator)(nil)
