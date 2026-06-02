package errors

import "google.golang.org/grpc/codes"

// ValidationOption configures validation errors
type ValidationOption func(*validationConfig)

// validationConfig holds validation error configuration
type validationConfig struct {
	details []Detail
	message string
}

// WithValidationMessage sets the overall validation message
func WithValidationMessage(message string) ValidationOption {
	return func(cfg *validationConfig) {
		cfg.message = message
	}
}

// WithRequiredField adds a required field error
func WithRequiredField(field string) ValidationOption {
	return func(cfg *validationConfig) {
		cfg.details = append(cfg.details, NewDetail(
			field,
			CodeMissingField,
			"This field is required",
			nil,
		))
	}
}

// WithInvalidFormat adds an invalid format error
func WithInvalidFormat(field string, value interface{}, expectedFormat string) ValidationOption {
	return func(cfg *validationConfig) {
		message := "Invalid format"
		if expectedFormat != "" {
			message += " (expected: " + expectedFormat + ")"
		}

		cfg.details = append(cfg.details, NewDetail(
			field,
			CodeInvalidFormat,
			message,
			value,
		))
	}
}

// WithOutOfRange adds an out of range error
func WithOutOfRange(field string, value interface{}, min, max interface{}) ValidationOption {
	return func(cfg *validationConfig) {
		cfg.details = append(cfg.details, NewDetail(
			field,
			CodeOutOfRange,
			"Value is out of allowed range",
			value,
		))
	}
}

// WithCustomValidation adds a custom field error
func WithCustomValidation(field string, code Code, message string, value interface{}) ValidationOption {
	return func(cfg *validationConfig) {
		cfg.details = append(cfg.details, NewDetail(field, code, message, value))
	}
}

// WithValidationDetail adds a pre-built detail
func WithValidationDetail(detail Detail) ValidationOption {
	return func(cfg *validationConfig) {
		if detail != nil {
			cfg.details = append(cfg.details, detail)
		}
	}
}

// WithValidationDetails adds multiple pre-built details
func WithValidationDetails(details ...Detail) ValidationOption {
	return func(cfg *validationConfig) {
		for _, detail := range details {
			if detail != nil {
				cfg.details = append(cfg.details, detail)
			}
		}
	}
}

// ValidationError creates a validation error using functional options
func ValidationError(opts ...ValidationOption) Error {
	cfg := &validationConfig{
		details: make([]Detail, 0),
		message: "Validation failed",
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// Return nil if no validation errors
	if len(cfg.details) == 0 {
		return nil
	}

	return New(CodeValidationFailed,
		WithMessage(cfg.message),
		WithHTTPStatus(400),
		WithGRPCCode(codes.InvalidArgument),
		WithDetails(cfg.details...),
	)
}
