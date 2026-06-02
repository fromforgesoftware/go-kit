package rest

import (
	"context"
	"io"
	"net/http"
	"strconv"

	"github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/jsonapi"
)

func JsonApiErrorEncoder(ctx context.Context, err error, w http.ResponseWriter) {
	// Resolve the wire status. A kit error built via apierrors.New(code,
	// ...) without WithHTTPStatus(...) reports HTTPStatus() == 0; feeding
	// that to net/http's WriteHeader panics. Fall back to 500 in that
	// case so the encoder never panics on an under-built error.
	statusCode := resolveStatus(err)

	// Marshal the error in JSON-API format into a buffer first so a marshal
	// failure can fall back cleanly instead of leaving a torn body in flight.
	// The same resolved status is threaded into the body so the JSON:API
	// error objects' `status` field matches the HTTP header.
	writeErr := writeBuffered(w, "application/vnd.api+json", statusCode, func(buf io.Writer) error {
		return jsonapi.MarshalErrors(buf, transformError(err, statusCode))
	})
	if writeErr != nil {
		http.Error(w, err.Error(), statusCode)
	}
}

// resolveStatus returns HTTPStatus() when it's > 0, otherwise 500.
// Keeps the wire response from inheriting an unset status.
func resolveStatus(err error) int {
	if apiErr, ok := errors.As(err); ok {
		if s := apiErr.HTTPStatus(); s > 0 {
			return s
		}
	}
	return http.StatusInternalServerError
}

// transformError converts a structured error into JSON API error objects.
// statusCode is the already-resolved wire status — callers pass it in so
// every error object's `status` field matches the HTTP header.
func transformError(err error, statusCode int) []*jsonapi.ErrorObject {
	// Try to extract our structured error first
	if apiErr, ok := errors.As(err); ok {
		return transformStructuredError(apiErr, statusCode)
	}

	// Fallback for unknown errors
	return []*jsonapi.ErrorObject{
		{
			Status: strconv.Itoa(statusCode),
			Code:   "INTERNAL_ERROR",
			Title:  "Internal Server Error",
			Detail: err.Error(),
		},
	}
}

// transformStructuredError converts our structured error into JSON API
// format. statusCode is the wire status the caller already resolved (see
// resolveStatus); we use it on every error object so body + header agree.
func transformStructuredError(err errors.Error, statusCode int) []*jsonapi.ErrorObject {
	status := strconv.Itoa(statusCode)
	var errorObjects []*jsonapi.ErrorObject

	// For field validation errors, only return field-level errors
	if len(err.Details()) > 0 && isFieldValidationError(err) {
		// Add field-level errors as separate error objects
		for _, detail := range err.Details() {
			fieldError := &jsonapi.ErrorObject{
				Status: status,
				Code:   detail.Code().String(),
				Title:  getErrorTitle(detail.Code()),
				Detail: detail.Message(),
				Source: &jsonapi.ErrorSource{
					Pointer: "/data/attributes/" + detail.Field(),
				},
			}

			// Add meta information including field name and invalid value
			fieldMeta := map[string]interface{}{
				"field": detail.Field(),
			}
			if detail.Value() != nil {
				fieldMeta["invalid_value"] = detail.Value()
			}
			fieldError.Meta = &fieldMeta

			errorObjects = append(errorObjects, fieldError)
		}
		return errorObjects
	}

	// Create the main error object for non-field validation errors
	mainError := &jsonapi.ErrorObject{
		Status: status,
		Code:   err.Code().String(),
		Title:  getErrorTitle(err.Code()),
		Detail: err.Message(),
	}

	// Add meta information if available
	meta := make(map[string]interface{})
	if err.RequestID() != "" {
		meta["request_id"] = err.RequestID()
	}
	if err.Service() != "" {
		meta["service"] = err.Service()
	}
	// if !err.Timestamp().IsZero() {
	// 	meta["timestamp"] = err.Timestamp()
	// }
	if len(meta) > 0 {
		mainError.Meta = &meta
	}

	errorObjects = append(errorObjects, mainError)

	// Add field-level errors as separate error objects (for complex errors)
	for _, detail := range err.Details() {
		fieldError := &jsonapi.ErrorObject{
			Status: status,
			Code:   detail.Code().String(),
			Title:  getErrorTitle(detail.Code()),
			Detail: detail.Message(),
			Source: &jsonapi.ErrorSource{
				Pointer: "/data/attributes/" + detail.Field(),
			},
		}

		// Add meta information including field name and invalid value
		fieldMeta := map[string]interface{}{
			"field": detail.Field(),
		}
		if detail.Value() != nil {
			fieldMeta["invalid_value"] = detail.Value()
		}
		fieldError.Meta = &fieldMeta

		errorObjects = append(errorObjects, fieldError)
	}

	return errorObjects
}

// isFieldValidationError checks if this is a pure field validation error
func isFieldValidationError(err errors.Error) bool {
	code := err.Code()
	return code == errors.CodeMissingField ||
		code == errors.CodeInvalidFormat ||
		code == errors.CodeOutOfRange ||
		code == errors.CodeValidationFailed
}

// getErrorTitle returns a human-readable title for error codes
func getErrorTitle(code errors.Code) string {
	switch code {
	case errors.CodeValidationFailed:
		return "Validation Failed"
	case errors.CodeInvalidArgument:
		return "Invalid Input"
	case errors.CodeMissingField:
		return "Missing Required Field"
	case errors.CodeInvalidFormat:
		return "Invalid Format"
	case errors.CodeOutOfRange:
		return "Value Out of Range"
	case errors.CodeNotFound:
		return "Resource Not Found"
	case errors.CodeAlreadyExists:
		return "Resource Already Exists"
	case errors.CodeConflict:
		return "Conflict"
	case errors.CodeUnauthenticated:
		return "Authentication Required"
	case errors.CodeForbidden:
		return "Access Forbidden"
	case errors.CodeInternalError:
		return "Internal Server Error"
	case errors.CodeServiceUnavailable:
		return "Service Unavailable"
	case errors.CodeTimeout:
		return "Request Timeout"
	case errors.CodeRateLimited:
		return "Too Many Requests"
	default:
		return "Error"
	}
}
