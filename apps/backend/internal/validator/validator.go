package validator

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

type Validator struct {
	validate *validator.Validate
}

func New() *Validator {
	v := validator.New(validator.WithRequiredStructEnabled())

	// Report the JSON name so the client sees the field it actually sent.
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
		if name == "" || name == "-" {
			return field.Name
		}
		return name
	})

	_ = v.RegisterValidation("password", validatePassword)

	return &Validator{validate: v}
}

// Struct validates and converts failures into a field error map suitable for a 422 response.
func (v *Validator) Struct(payload any) error {
	if err := v.validate.Struct(payload); err != nil {
		var invalid *validator.InvalidValidationError
		if ok := asInvalid(err, &invalid); ok {
			return utils.WrapDomainError(utils.ErrValidation, "Validation failed", err)
		}

		fields := map[string]string{}
		var validationErrors validator.ValidationErrors
		if asValidationErrors(err, &validationErrors) {
			for _, fieldErr := range validationErrors {
				fields[fieldErr.Field()] = message(fieldErr)
			}
		}
		return utils.NewFieldError("Validation failed", fields)
	}
	return nil
}

func asInvalid(err error, target **validator.InvalidValidationError) bool {
	casted, ok := err.(*validator.InvalidValidationError)
	if ok {
		*target = casted
	}
	return ok
}

func asValidationErrors(err error, target *validator.ValidationErrors) bool {
	casted, ok := err.(validator.ValidationErrors)
	if ok {
		*target = casted
	}
	return ok
}

func message(fieldErr validator.FieldError) string {
	field := fieldErr.Field()
	switch fieldErr.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, fieldErr.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, fieldErr.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, strings.ReplaceAll(fieldErr.Param(), " ", ", "))
	case "password":
		return fmt.Sprintf("%s must be at least 8 characters and contain an upper case letter, a lower case letter and a digit", field)
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, fieldErr.Param())
	case "datetime":
		return fmt.Sprintf("%s must use the format %s", field, fieldErr.Param())
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}

func validatePassword(field validator.FieldLevel) bool {
	value := field.Field().String()
	if len(value) < 8 {
		return false
	}
	var hasUpper, hasLower, hasDigit bool
	for _, char := range value {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		}
	}
	return hasUpper && hasLower && hasDigit
}
