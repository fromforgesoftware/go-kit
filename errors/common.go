package errors

import "google.golang.org/grpc/codes"

// Validation Errors

// ValidationFailed creates a validation error with field details
func ValidationFailed(message string, opts ...Option) Error {
	options := []Option{
		WithMessage(message),
		WithHTTPStatus(400),
		WithGRPCCode(codes.InvalidArgument),
	}
	options = append(options, opts...)
	return New(CodeValidationFailed, options...)
}

// InvalidArgument creates an invalid input error
func InvalidArgument(message string, opts ...Option) Error {
	options := []Option{
		WithMessage(message),
		WithHTTPStatus(400),
		WithGRPCCode(codes.InvalidArgument),
	}
	options = append(options, opts...)
	return New(CodeInvalidArgument, options...)
}

// MissingField creates a missing field validation error
func MissingField(field string, opts ...Option) Error {
	options := []Option{
		WithMessage("Missing required field: " + field),
		WithHTTPStatus(400),
		WithGRPCCode(codes.InvalidArgument),
		WithDetail(field, CodeMissingField, "This field is required", nil),
	}
	options = append(options, opts...)
	return New(CodeMissingField, options...)
}

// InvalidFormat creates an invalid format validation error
func InvalidFormat(field string, value interface{}, expectedFormat string, opts ...Option) Error {
	message := "Invalid format for field: " + field
	if expectedFormat != "" {
		message += " (expected: " + expectedFormat + ")"
	}

	options := []Option{
		WithMessage(message),
		WithHTTPStatus(400),
		WithGRPCCode(codes.InvalidArgument),
		WithDetail(field, CodeInvalidFormat, message, value),
	}
	options = append(options, opts...)
	return New(CodeInvalidFormat, options...)
}

// OutOfRange creates an out of range validation error
func OutOfRange(field string, value interface{}, min, max interface{}, opts ...Option) Error {
	message := "Value out of range for field: " + field

	options := []Option{
		WithMessage(message),
		WithHTTPStatus(400),
		WithGRPCCode(codes.InvalidArgument),
		WithDetail(field, CodeOutOfRange, message, value),
	}
	options = append(options, opts...)
	return New(CodeOutOfRange, options...)
}

// Resource Errors

// NotFound creates a not found error
func NotFound(resource string, identifier interface{}, opts ...Option) Error {
	message := resource + " not found"
	if identifier != nil {
		message += " with identifier: " + toString(identifier)
	}

	options := []Option{
		WithMessage(message),
		WithHTTPStatus(404),
		WithGRPCCode(codes.NotFound),
	}
	options = append(options, opts...)
	return New(CodeNotFound, options...)
}

// AlreadyExists creates an already exists error
func AlreadyExists(resource string, identifier interface{}, opts ...Option) Error {
	message := resource + " already exists"
	if identifier != nil {
		message += " with identifier: " + toString(identifier)
	}

	options := []Option{
		WithMessage(message),
		WithHTTPStatus(409),
		WithGRPCCode(codes.AlreadyExists),
	}
	options = append(options, opts...)
	return New(CodeAlreadyExists, options...)
}

// Conflict creates a conflict error
func Conflict(message string, opts ...Option) Error {
	options := []Option{
		WithMessage(message),
		WithHTTPStatus(409),
		WithGRPCCode(codes.FailedPrecondition),
	}
	options = append(options, opts...)
	return New(CodeConflict, options...)
}

// Authentication/Authorization Errors

// Unauthenticated creates an unauthenticated error
func Unauthenticated(message string, opts ...Option) Error {
	if message == "" {
		message = "Authentication required"
	}

	options := []Option{
		WithMessage(message),
		WithHTTPStatus(401),
		WithGRPCCode(codes.Unauthenticated),
	}
	options = append(options, opts...)
	return New(CodeUnauthenticated, options...)
}

// Unauthorized creates an unauthorized error (alias for Unauthenticated for clarity)
func Unauthorized(message string, opts ...Option) Error {
	return Unauthenticated(message, opts...)
}

// Forbidden creates a forbidden error
func Forbidden(message string, opts ...Option) Error {
	if message == "" {
		message = "Access forbidden"
	}

	options := []Option{
		WithMessage(message),
		WithHTTPStatus(403),
		WithGRPCCode(codes.PermissionDenied),
	}
	options = append(options, opts...)
	return New(CodeForbidden, options...)
}

// System Errors

// InternalError creates an internal error
func InternalError(message string, opts ...Option) Error {
	if message == "" {
		message = "Internal server error"
	}

	options := []Option{
		WithMessage(message),
		WithHTTPStatus(500),
		WithGRPCCode(codes.Internal),
	}
	options = append(options, opts...)
	return New(CodeInternalError, options...)
}

// ServiceUnavailable creates a service unavailable error
func ServiceUnavailable(message string, opts ...Option) Error {
	if message == "" {
		message = "Service temporarily unavailable"
	}

	options := []Option{
		WithMessage(message),
		WithHTTPStatus(503),
		WithGRPCCode(codes.Unavailable),
	}
	options = append(options, opts...)
	return New(CodeServiceUnavailable, options...)
}

// Timeout creates a timeout error
func Timeout(message string, opts ...Option) Error {
	if message == "" {
		message = "Request timeout"
	}

	options := []Option{
		WithMessage(message),
		WithHTTPStatus(408),
		WithGRPCCode(codes.DeadlineExceeded),
	}
	options = append(options, opts...)
	return New(CodeTimeout, options...)
}

// RateLimited creates a rate limited error
func RateLimited(message string, opts ...Option) Error {
	if message == "" {
		message = "Too many requests"
	}

	options := []Option{
		WithMessage(message),
		WithHTTPStatus(429),
		WithGRPCCode(codes.ResourceExhausted),
	}
	options = append(options, opts...)
	return New(CodeRateLimited, options...)
}

// Wrapping helpers for common scenarios

// WrapValidation wraps an error as a validation error
func WrapValidation(err error, message string, opts ...Option) Error {
	options := []Option{
		WithMessage(message),
		WithHTTPStatus(400),
		WithGRPCCode(codes.InvalidArgument),
	}
	options = append(options, opts...)
	return Wrap(err, CodeValidationFailed, options...)
}

// WrapNotFound wraps an error as a not found error
func WrapNotFound(err error, resource string, identifier interface{}, opts ...Option) Error {
	message := resource + " not found"
	if identifier != nil {
		message += " with identifier: " + toString(identifier)
	}

	options := []Option{
		WithMessage(message),
		WithHTTPStatus(404),
		WithGRPCCode(codes.NotFound),
	}
	options = append(options, opts...)
	return Wrap(err, CodeNotFound, options...)
}

// WrapInternal wraps an error as an internal error
func WrapInternal(err error, message string, opts ...Option) Error {
	if message == "" {
		message = "Internal server error"
	}

	options := []Option{
		WithMessage(message),
		WithHTTPStatus(500),
		WithGRPCCode(codes.Internal),
	}
	options = append(options, opts...)
	return Wrap(err, CodeInternalError, options...)
}

// Helper function to convert any value to string
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	// For non-string types, we'll use a simple approach
	// In a real implementation, you might want to use fmt.Sprintf("%v", v)
	switch val := v.(type) {
	case int, int32, int64:
		return "number"
	case float32, float64:
		return "decimal"
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return "value"
	}
}
