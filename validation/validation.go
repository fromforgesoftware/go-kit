// Package validation provides struct-tag-driven input validation used by
// the REST + gRPC handler factories.
package validation

import (
	"fmt"

	"github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/resource"
)

func ValidateIdentifier(kind resource.Type) func(resource.Identifier) error {
	return func(id resource.Identifier) error {
		if id.Type() != kind {
			return errors.InvalidArgument(fmt.Sprintf("invalid identifier type: expected %s, got %s", kind, id.Type()))
		}
		if id.ID() == "" {
			return errors.InvalidArgument("identifier id cannot be empty")
		}
		return nil
	}
}

func NotEmptyStringValidator(fieldName string) func(string) error {
	return func(value string) error {
		if value == "" {
			return errors.InvalidArgument(fmt.Sprintf("field %s cannot be empty", fieldName))
		}
		return nil
	}
}

func EnumValidator[T comparable](fieldName string, allowed ...T) func(T) error {
	return func(value T) error {
		for _, a := range allowed {
			if value == a {
				return nil
			}
		}
		return errors.InvalidArgument(fmt.Sprintf("field %s has invalid value", fieldName))
	}
}
