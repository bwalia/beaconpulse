// Package validate wraps go-playground/validator to produce Beacon's structured
// validation errors. Handlers decode a request DTO and call Struct; any failure
// is returned as an *apperror.Error with per-field messages, giving the frontend
// everything it needs to highlight the offending inputs.
package validate

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"

	"beacon/internal/platform/apperror"
)

// Validator validates struct DTOs using tags.
type Validator struct {
	v *validator.Validate
}

var (
	shared     *Validator
	sharedOnce sync.Once
)

// New builds a Validator that reports errors using the struct's `json` tag name
// so field references match the API contract rather than Go field names.
func New() *Validator {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
	return &Validator{v: v}
}

// Default returns a lazily-initialized shared Validator. Convenient for
// packages that do not receive one via injection.
func Default() *Validator {
	sharedOnce.Do(func() { shared = New() })
	return shared
}

// Struct validates s and returns nil on success or an *apperror.Error with
// field-level detail on failure.
func (val *Validator) Struct(s any) error {
	err := val.v.Struct(s)
	if err == nil {
		return nil
	}
	var invalid *validator.InvalidValidationError
	if ok := asInvalid(err, &invalid); ok {
		return apperror.Internal(err)
	}

	verrs := err.(validator.ValidationErrors)
	fields := make([]apperror.FieldError, 0, len(verrs))
	for _, fe := range verrs {
		fields = append(fields, apperror.FieldError{
			Field:   fe.Field(),
			Message: message(fe),
		})
	}
	return apperror.Validation("one or more fields are invalid", fields...)
}

func asInvalid(err error, target **validator.InvalidValidationError) bool {
	if e, ok := err.(*validator.InvalidValidationError); ok {
		*target = e
		return true
	}
	return false
}

// message renders a human-readable message for a single field violation.
func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "url":
		return "must be a valid URL"
	case "min":
		return fmt.Sprintf("must be at least %s", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", fe.Param())
	case "gte":
		return fmt.Sprintf("must be greater than or equal to %s", fe.Param())
	case "lte":
		return fmt.Sprintf("must be less than or equal to %s", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of: %s", fe.Param())
	case "uuid":
		return "must be a valid UUID"
	case "hostname_rfc1123", "fqdn":
		return "must be a valid hostname"
	default:
		return fmt.Sprintf("failed %q validation", fe.Tag())
	}
}
