package errors

// Code represents an internal error code
type Code string

// String returns the string representation of the error code
func (c Code) String() string {
	return string(c)
}

// Common error codes
const (
	// Validation errors
	CodeValidationFailed Code = "VALIDATION_FAILED"
	CodeInvalidArgument  Code = "INVALID_ARGUMENT"
	CodeMissingField     Code = "MISSING_FIELD"
	CodeInvalidFormat    Code = "INVALID_FORMAT"
	CodeOutOfRange       Code = "OUT_OF_RANGE"

	// Resource errors
	CodeNotFound      Code = "NOT_FOUND"
	CodeAlreadyExists Code = "ALREADY_EXISTS"
	CodeConflict      Code = "CONFLICT"
	CodeTaken         Code = "TAKEN"

	// Authentication/Authorization errors
	CodeUnauthenticated Code = "UNAUTHENTICATED"
	CodeUnauthorized    Code = "UNAUTHORIZED"
	CodeForbidden       Code = "FORBIDDEN"

	// System errors
	CodeInternalError      Code = "INTERNAL_ERROR"
	CodeServiceUnavailable Code = "SERVICE_UNAVAILABLE"
	CodeTimeout            Code = "TIMEOUT"
	CodeRateLimited        Code = "RATE_LIMITED"

	// Business logic errors
	CodeBusinessRuleViolation Code = "BUSINESS_RULE_VIOLATION"
	CodeOperationNotAllowed   Code = "OPERATION_NOT_ALLOWED"
	CodeResourceLocked        Code = "RESOURCE_LOCKED"

	// Rate limiting and capacity
	CodeQuotaExceeded    Code = "QUOTA_EXCEEDED"
	CodeCapacityExceeded Code = "CAPACITY_EXCEEDED"

	// Data integrity
	CodeDataCorruption   Code = "DATA_CORRUPTION"
	CodeChecksumMismatch Code = "CHECKSUM_MISMATCH"

	// Network and connectivity
	CodeNetworkError        Code = "NETWORK_ERROR"
	CodeConnectionFailed    Code = "CONNECTION_FAILED"
	CodeDNSResolutionFailed Code = "DNS_RESOLUTION_FAILED"

	// File and storage
	CodeFileNotFound      Code = "FILE_NOT_FOUND"
	CodeStorageError      Code = "STORAGE_ERROR"
	CodeInsufficientSpace Code = "INSUFFICIENT_SPACE"

	// Configuration and setup
	CodeConfigurationError Code = "CONFIGURATION_ERROR"
	CodeMissingDependency  Code = "MISSING_DEPENDENCY"
	CodeVersionMismatch    Code = "VERSION_MISMATCH"

	// Resource state errors
	CodeGone               Code = "GONE"
	CodePreconditionFailed Code = "PRECONDITION_FAILED"

	// External service errors
	CodeExternalService Code = "EXTERNAL_SERVICE_ERROR"
	CodeDatabaseError   Code = "DATABASE_ERROR"
)
