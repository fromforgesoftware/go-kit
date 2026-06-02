package filter

import (
	"fmt"
	"strconv"

	"github.com/fromforgesoftware/go-kit/errors"
	"github.com/google/uuid"
)

func ValidateUUID(f FieldFilter[any]) error {
	val := f.Value()
	switch v := val.(type) {
	case string:
		if _, err := uuid.Parse(v); err != nil {
			return errors.InvalidArgument(fmt.Sprintf("invalid UUID for field %s", f.Name()))
		}
	case []string:
		for _, id := range v {
			if _, err := uuid.Parse(id); err != nil {
				return errors.InvalidArgument(fmt.Sprintf("invalid UUID in list for field %s", f.Name()))
			}
		}
	default:
		// Attempt to convert to string if possible, or fail
		s := fmt.Sprintf("%v", v)
		if _, err := uuid.Parse(s); err != nil {
			return errors.InvalidArgument(fmt.Sprintf("invalid value type for field %s, expected UUID", f.Name()))
		}
	}
	return nil
}

func ValidateArrayOrSingleField[T any](f FieldFilter[any]) error {
	// This acts as a type check
	val := f.Value()
	switch val.(type) {
	case T:
		return nil
	case []T:
		return nil
	}
	return errors.InvalidArgument(fmt.Sprintf("invalid type for field %s", f.Name()))
}

// ValidateTyped validates that a field filter's value is of the specified type T
func ValidateTyped[T any](f FieldFilter[any]) error {
	val := f.Value()
	switch val.(type) {
	case T:
		return nil
	case []T:
		return nil
	}
	return errors.InvalidArgument(fmt.Sprintf("invalid type for field %s, expected %T", f.Name(), *new(T)))
}

// ValidateNotZero validates that a field filter's value is not a zero value
func ValidateNotZero(f FieldFilter[any]) error {
	val := f.Value()
	if val == nil {
		return errors.InvalidArgument(fmt.Sprintf("field %s cannot be nil", f.Name()))
	}

	// Check for zero values based on type
	switch v := val.(type) {
	case string:
		if v == "" {
			return errors.InvalidArgument(fmt.Sprintf("field %s cannot be empty", f.Name()))
		}
	case int, int8, int16, int32, int64:
		if v == 0 {
			return errors.InvalidArgument(fmt.Sprintf("field %s cannot be zero", f.Name()))
		}
	case uint, uint8, uint16, uint32, uint64:
		if v == 0 {
			return errors.InvalidArgument(fmt.Sprintf("field %s cannot be zero", f.Name()))
		}
	case float32, float64:
		if v == 0.0 {
			return errors.InvalidArgument(fmt.Sprintf("field %s cannot be zero", f.Name()))
		}
	case bool:
		// bool zero value is false, but false might be valid
		return nil
	}
	return nil
}

// ValidateValOneOf returns a validator that checks if the value is one of the allowed values
func ValidateValOneOf[T comparable](allowed ...T) func(FieldFilter[any]) error {
	return func(f FieldFilter[any]) error {
		val := f.Value()

		// Try to cast to T
		typedVal, ok := val.(T)
		if !ok {
			return errors.InvalidArgument(fmt.Sprintf("field %s has invalid type, expected %T", f.Name(), *new(T)))
		}

		// Check if value is in allowed list
		for _, allowedVal := range allowed {
			if typedVal == allowedVal {
				return nil
			}
		}

		return errors.InvalidArgument(fmt.Sprintf("field %s has invalid value, must be one of the allowed values", f.Name()))
	}
}

// AnyValid returns a validator that passes if at least one of the validators passes
func AnyValid(validators ...func(FieldFilter[any]) error) func(FieldFilter[any]) error {
	return func(f FieldFilter[any]) error {
		var lastErr error
		for _, v := range validators {
			if err := v(f); err == nil {
				return nil
			} else {
				lastErr = err
			}
		}
		return lastErr
	}
}

func ValidateNil(f FieldFilter[any]) error {
	if f.Value() != nil {
		return errors.InvalidArgument(fmt.Sprintf("field %s must be nil", f.Name()))
	}
	return nil
}

func ValidateIntegerString(f FieldFilter[any]) error {
	val := f.Value()
	s, ok := val.(string)
	if !ok {
		// Handle slice of strings?
		if sl, ok := val.([]string); ok {
			for _, item := range sl {
				if _, err := strconv.Atoi(item); err != nil {
					return errors.InvalidArgument(fmt.Sprintf("invalid integer string in list for field %s", f.Name()))
				}
			}
			return nil
		}
		return errors.InvalidArgument(fmt.Sprintf("invalid type for field %s, expected string", f.Name()))
	}
	if _, err := strconv.Atoi(s); err != nil {
		return errors.InvalidArgument(fmt.Sprintf("invalid integer string for field %s", f.Name()))
	}
	return nil
}
