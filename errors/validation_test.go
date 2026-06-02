package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

// Test constants for validation tests
const (
	CodeCustomCode Code = "CUSTOM_CODE"
	CodeCode1      Code = "CODE1"
	CodeCode2      Code = "CODE2"
)

func TestValidationError(t *testing.T) {
	tests := []struct {
		name          string
		opts          []ValidationOption
		expectNil     bool
		expectMessage string
		expectCode    Code
		expectHTTP    int
		expectGRPC    codes.Code
		expectDetails int
	}{
		{
			name:      "no validation options returns nil",
			opts:      []ValidationOption{},
			expectNil: true,
		},
		{
			name: "single required field error",
			opts: []ValidationOption{
				WithRequiredField("email"),
			},
			expectMessage: "Validation failed",
			expectCode:    CodeValidationFailed,
			expectHTTP:    400,
			expectGRPC:    codes.InvalidArgument,
			expectDetails: 1,
		},
		{
			name: "multiple validation errors",
			opts: []ValidationOption{
				WithValidationMessage("User registration failed"),
				WithRequiredField("email"),
				WithInvalidFormat("phone", "+1234", "E.164 format"),
				WithOutOfRange("age", 15, 18, 65),
			},
			expectMessage: "User registration failed",
			expectCode:    CodeValidationFailed,
			expectHTTP:    400,
			expectGRPC:    codes.InvalidArgument,
			expectDetails: 3,
		},
		{
			name: "custom validation error",
			opts: []ValidationOption{
				WithCustomValidation("username", CodeAlreadyExists, "Username already taken", "john_doe"),
			},
			expectMessage: "Validation failed",
			expectCode:    CodeValidationFailed,
			expectHTTP:    400,
			expectGRPC:    codes.InvalidArgument,
			expectDetails: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidationError(tt.opts...)

			if tt.expectNil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.Equal(t, tt.expectMessage, result.Message())
			assert.Equal(t, tt.expectCode, result.Code())
			assert.Equal(t, tt.expectHTTP, result.HTTPStatus())
			assert.Equal(t, tt.expectGRPC, result.GRPCCode())
			assert.Len(t, result.Details(), tt.expectDetails)

			// Check that all details are properly set
			for _, detail := range result.Details() {
				assert.NotEmpty(t, detail.Field())
				assert.NotEmpty(t, detail.Code())
				assert.NotEmpty(t, detail.Message())
			}
		})
	}
}

func TestValidationOptions(t *testing.T) {
	t.Run("WithValidationMessage", func(t *testing.T) {
		err := ValidationError(
			WithValidationMessage("Custom validation message"),
			WithRequiredField("test"),
		)

		require.NotNil(t, err)
		assert.Equal(t, "Custom validation message", err.Message())
	})

	t.Run("WithRequiredField", func(t *testing.T) {
		err := ValidationError(WithRequiredField("email"))

		require.NotNil(t, err)
		details := err.Details()
		require.Len(t, details, 1)

		detail := details[0]
		assert.Equal(t, "email", detail.Field())
		assert.Equal(t, CodeMissingField, detail.Code())
		assert.Equal(t, "This field is required", detail.Message())
		assert.Nil(t, detail.Value())
	})

	t.Run("WithInvalidFormat", func(t *testing.T) {
		err := ValidationError(WithInvalidFormat("phone", "+1234", "E.164"))

		require.NotNil(t, err)
		details := err.Details()
		require.Len(t, details, 1)

		detail := details[0]
		assert.Equal(t, "phone", detail.Field())
		assert.Equal(t, CodeInvalidFormat, detail.Code())
		assert.Equal(t, "Invalid format (expected: E.164)", detail.Message())
		assert.Equal(t, "+1234", detail.Value())
	})

	t.Run("WithOutOfRange", func(t *testing.T) {
		err := ValidationError(WithOutOfRange("age", 15, 18, 65))

		require.NotNil(t, err)
		details := err.Details()
		require.Len(t, details, 1)

		detail := details[0]
		assert.Equal(t, "age", detail.Field())
		assert.Equal(t, CodeOutOfRange, detail.Code())
		assert.Equal(t, "Value is out of allowed range", detail.Message())
		assert.Equal(t, 15, detail.Value())
	})

	t.Run("WithCustomValidation", func(t *testing.T) {
		err := ValidationError(WithCustomValidation("username", CodeTaken, "Username already exists", "john"))

		require.NotNil(t, err)
		details := err.Details()
		require.Len(t, details, 1)

		detail := details[0]
		assert.Equal(t, "username", detail.Field())
		assert.Equal(t, CodeTaken, detail.Code())
		assert.Equal(t, "Username already exists", detail.Message())
		assert.Equal(t, "john", detail.Value())
	})

	t.Run("WithValidationDetail", func(t *testing.T) {
		customDetail := NewDetail("custom", CodeCustomCode, "Custom message", "value")
		err := ValidationError(WithValidationDetail(customDetail))

		require.NotNil(t, err)
		details := err.Details()
		require.Len(t, details, 1)

		detail := details[0]
		assert.Equal(t, "custom", detail.Field())
		assert.Equal(t, CodeCustomCode, detail.Code())
		assert.Equal(t, "Custom message", detail.Message())
		assert.Equal(t, "value", detail.Value())
	})

	t.Run("WithValidationDetails", func(t *testing.T) {
		detail1 := NewDetail("field1", CodeCode1, "Message 1", "value1")
		detail2 := NewDetail("field2", CodeCode2, "Message 2", "value2")

		err := ValidationError(WithValidationDetails(detail1, detail2))

		require.NotNil(t, err)
		details := err.Details()
		require.Len(t, details, 2)

		// Check first detail
		assert.Equal(t, "field1", details[0].Field())
		assert.Equal(t, CodeCode1, details[0].Code())

		// Check second detail
		assert.Equal(t, "field2", details[1].Field())
		assert.Equal(t, CodeCode2, details[1].Code())
	})
}

func TestPreBuiltErrors(t *testing.T) {
	t.Run("NotFound", func(t *testing.T) {
		err := NotFound("user", "123")

		require.NotNil(t, err)
		assert.Equal(t, CodeNotFound, err.Code())
		assert.Equal(t, "user not found with identifier: 123", err.Message())
		assert.Equal(t, 404, err.HTTPStatus())
		assert.Equal(t, codes.NotFound, err.GRPCCode())
	})

	t.Run("Conflict", func(t *testing.T) {
		err := Conflict("Resource already exists")

		require.NotNil(t, err)
		assert.Equal(t, CodeConflict, err.Code())
		assert.Equal(t, "Resource already exists", err.Message())
		assert.Equal(t, 409, err.HTTPStatus())
		assert.Equal(t, codes.FailedPrecondition, err.GRPCCode())
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		err := Unauthenticated("Invalid credentials")

		require.NotNil(t, err)
		assert.Equal(t, CodeUnauthenticated, err.Code())
		assert.Equal(t, "Invalid credentials", err.Message())
		assert.Equal(t, 401, err.HTTPStatus())
		assert.Equal(t, codes.Unauthenticated, err.GRPCCode())
	})

	t.Run("Forbidden", func(t *testing.T) {
		err := Forbidden("Insufficient permissions")

		require.NotNil(t, err)
		assert.Equal(t, CodeForbidden, err.Code())
		assert.Equal(t, "Insufficient permissions", err.Message())
		assert.Equal(t, 403, err.HTTPStatus())
		assert.Equal(t, codes.PermissionDenied, err.GRPCCode())
	})

	t.Run("InternalError", func(t *testing.T) {
		err := InternalError("Database connection failed")

		require.NotNil(t, err)
		assert.Equal(t, CodeInternalError, err.Code())
		assert.Equal(t, "Database connection failed", err.Message())
		assert.Equal(t, 500, err.HTTPStatus())
		assert.Equal(t, codes.Internal, err.GRPCCode())
	})

	t.Run("ServiceUnavailable", func(t *testing.T) {
		err := ServiceUnavailable("Service temporarily unavailable")

		require.NotNil(t, err)
		assert.Equal(t, CodeServiceUnavailable, err.Code())
		assert.Equal(t, "Service temporarily unavailable", err.Message())
		assert.Equal(t, 503, err.HTTPStatus())
		assert.Equal(t, codes.Unavailable, err.GRPCCode())
	})
}
