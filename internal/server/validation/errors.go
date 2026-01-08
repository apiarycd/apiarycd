package validation

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/samber/lo"
)

// Error represents a validation error with field details.
type Error struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Value   string `json:"value,omitempty"`
	Message string `json:"message"`
}

// Errors represents multiple validation errors.
type Errors []Error

// NewErrors converts validator errors to ValidationErrors.
func NewErrors(err error) error {
	if errs, ok := lo.ErrorsAs[validator.ValidationErrors](err); ok {
		validationErrs := make([]Error, 0, len(errs))
		for _, e := range errs {
			validationErr := Error{
				Field:   e.Field(),
				Tag:     e.Tag(),
				Value:   fmt.Sprintf("%v", e.Value()),
				Message: getMessageForTag(e.Tag(), e.Field(), e.Param()),
			}
			validationErrs = append(validationErrs, validationErr)
		}
		return Errors(validationErrs)
	}
	return err
}

func (ve Errors) Error() string {
	messages := make([]string, 0, len(ve))
	for _, err := range ve {
		messages = append(messages, fmt.Sprintf("%s: %s", err.Field, err.Message))
	}
	return fmt.Sprintf("validation failed: %s", strings.Join(messages, "; "))
}

// getMessageForTag returns a human-readable message for a validation tag.
func getMessageForTag(tag, field, param string) string {
	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s", field, param)
	case "max":
		return fmt.Sprintf("%s must be at most %s", field, param)
	case "url":
		return fmt.Sprintf("%s must be a valid URL", field)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, strings.ReplaceAll(param, " ", ", "))
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}
