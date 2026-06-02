package errors

import (
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error represents a structured error with rich context
type Error interface {
	error // Embeds standard error interface

	// Core error information
	Code() Code      // Internal error code (e.g., VALIDATION_FAILED)
	Message() string // Human-readable message

	// Protocol-specific codes
	HTTPStatus() int      // HTTP status code (400, 500, etc.)
	GRPCCode() codes.Code // gRPC status code

	// Context and details
	Details() []Detail // Field-level or additional error details
	Cause() error      // Underlying error (for wrapping)

	// Metadata
	Timestamp() time.Time // When the error occurred
	RequestID() string    // Request correlation ID
	Service() string      // Service that generated the error
}

// Detail represents a specific error detail (e.g., field validation error)
type Detail interface {
	Field() string      // Field name (if applicable)
	Code() Code         // Detail-specific error code
	Message() string    // Detail-specific message
	Value() interface{} // The value that caused the error
}

// errorImpl is the concrete implementation of Error
type errorImpl struct {
	code       Code
	message    string
	httpStatus int
	grpcCode   codes.Code
	details    []Detail
	cause      error
	timestamp  time.Time
	requestID  string
	service    string
}

// detailImpl is the concrete implementation of Detail
type detailImpl struct {
	field   string
	code    Code
	message string
	value   interface{}
}

// Error returns the error message
func (e *errorImpl) Error() string {
	if e.message != "" {
		return e.message
	}
	return e.code.String()
}

// Code returns the internal error code
func (e *errorImpl) Code() Code {
	return e.code
}

// Message returns the human-readable message
func (e *errorImpl) Message() string {
	return e.message
}

// HTTPStatus returns the HTTP status code
func (e *errorImpl) HTTPStatus() int {
	return e.httpStatus
}

// GRPCCode returns the gRPC status code
func (e *errorImpl) GRPCCode() codes.Code {
	return e.grpcCode
}

// GRPCStatus returns the gRPC Status representation of the error
func (e *errorImpl) GRPCStatus() *status.Status {
	return status.New(e.grpcCode, e.message)
}

// Details returns the error details
func (e *errorImpl) Details() []Detail {
	return e.details
}

// Cause returns the underlying error
func (e *errorImpl) Cause() error {
	return e.cause
}

// Timestamp returns when the error occurred
func (e *errorImpl) Timestamp() time.Time {
	return e.timestamp
}

// RequestID returns the request correlation ID
func (e *errorImpl) RequestID() string {
	return e.requestID
}

// Service returns the service that generated the error
func (e *errorImpl) Service() string {
	return e.service
}

// Unwrap returns the underlying error for Go 1.13+ error unwrapping
func (e *errorImpl) Unwrap() error {
	return e.cause
}

// Is implements error comparison for Go 1.13+ errors.Is functionality
// Two errors are considered equal if they have the same error code
func (e *errorImpl) Is(target error) bool {
	if t, ok := target.(Error); ok {
		return e.Code() == t.Code()
	}
	return false
}

// Implementation of Detail interface
func (d *detailImpl) Field() string {
	return d.field
}

func (d *detailImpl) Code() Code {
	return d.code
}

func (d *detailImpl) Message() string {
	return d.message
}

func (d *detailImpl) Value() interface{} {
	return d.value
}

// Option represents a functional option for configuring an Error
type Option func(*errorImpl)

// New creates a new Error with the given code and options
func New(code Code, opts ...Option) Error {
	e := &errorImpl{
		code:      code,
		timestamp: time.Now(),
		grpcCode:  codes.Unknown,
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Wrap wraps an existing error with additional context
func Wrap(err error, code Code, opts ...Option) Error {
	e := &errorImpl{
		code:      code,
		cause:     err,
		timestamp: time.Now(),
		grpcCode:  codes.Unknown,
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Functional options for configuring errors

// WithMessage sets the human-readable error message
func WithMessage(message string) Option {
	return func(e *errorImpl) {
		e.message = message
	}
}

// WithHTTPStatus sets the HTTP status code
func WithHTTPStatus(status int) Option {
	return func(e *errorImpl) {
		e.httpStatus = status
	}
}

// WithGRPCCode sets the gRPC status code
func WithGRPCCode(code codes.Code) Option {
	return func(e *errorImpl) {
		e.grpcCode = code
	}
}

// WithDetail adds a detail to the error
func WithDetail(field string, code Code, message string, value interface{}) Option {
	return func(e *errorImpl) {
		detail := &detailImpl{
			field:   field,
			code:    code,
			message: message,
			value:   value,
		}
		e.details = append(e.details, detail)
	}
}

// WithDetails adds multiple details to the error
func WithDetails(details ...Detail) Option {
	return func(e *errorImpl) {
		e.details = append(e.details, details...)
	}
}

// WithRequestID sets the request correlation ID
func WithRequestID(requestID string) Option {
	return func(e *errorImpl) {
		e.requestID = requestID
	}
}

// WithService sets the service that generated the error
func WithService(service string) Option {
	return func(e *errorImpl) {
		e.service = service
	}
}

// NewDetail creates a new Detail
func NewDetail(field string, code Code, message string, value interface{}) Detail {
	return &detailImpl{
		field:   field,
		code:    code,
		message: message,
		value:   value,
	}
}

// Is checks if the error matches the target error code
func Is(err error, code Code) bool {
	if e, ok := err.(Error); ok {
		return e.Code() == code
	}
	return false
}

// As extracts an Error from the error chain
func As(err error) (Error, bool) {
	if e, ok := err.(Error); ok {
		return e, true
	}
	return nil, false
}

// GetHTTPStatus extracts HTTP status from error, defaults to 500
func GetHTTPStatus(err error) int {
	if e, ok := As(err); ok {
		if status := e.HTTPStatus(); status != 0 {
			return status
		}
	}
	return 500
}

// GetGRPCCode extracts gRPC code from error, defaults to Unknown
func GetGRPCCode(err error) codes.Code {
	if e, ok := As(err); ok {
		return e.GRPCCode()
	}
	return codes.Unknown
}
