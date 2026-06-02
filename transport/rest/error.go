package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	kiterrors "github.com/fromforgesoftware/go-kit/errors"
)

// StatusCoder is checked by DefaultErrorEncoder. If an error value implements
// StatusCoder, the StatusCode will be used when encoding the error. By default,
// StatusInternalServerError (500) is used.
type StatusCoder interface {
	StatusCode() int
}

// Headerer is checked by DefaultErrorEncoder. If an error value implements
// Headerer, the provided headers will be applied to the response writer, after
// the Content-Type is set.
type Headerer interface {
	Headers() http.Header
}

// statusFromError returns the best HTTP status the encoder can derive
// from err. Order of precedence:
//
//  1. StatusCoder.StatusCode() — the legacy interface, kept for callers
//     that wrap their own typed errors.
//  2. kit/errors Error.HTTPStatus() — every error built via the helpers
//     in kit/errors/common.go (Unauthenticated, NotFound, Conflict, …)
//     and any New(...) with WithHTTPStatus(...). Falling back via
//     errors.As covers wrapped chains.
//
// Returns 500 when neither is set.
func statusFromError(err error) int {
	if sc, ok := err.(StatusCoder); ok && sc.StatusCode() > 0 {
		return sc.StatusCode()
	}
	var apiErr kiterrors.Error
	if errors.As(err, &apiErr) {
		if s := apiErr.HTTPStatus(); s > 0 {
			return s
		}
	}
	return http.StatusInternalServerError
}

// DefaultErrorEncoder encodes errors to HTTP responses. It does not log; an
// error-logging middleware in the chain is the right place to attribute
// errors to a request_id / span.
func DefaultErrorEncoder(_ context.Context, err error, w http.ResponseWriter) {
	contentType, body := "text/plain; charset=utf-8", []byte(err.Error())
	if marshaler, ok := err.(json.Marshaler); ok {
		if jsonBody, marshalErr := marshaler.MarshalJSON(); marshalErr == nil {
			contentType, body = "application/json; charset=utf-8", jsonBody
		}
	}
	w.Header().Set("Content-Type", contentType)
	if headerer, ok := err.(Headerer); ok {
		for k, values := range headerer.Headers() {
			for _, v := range values {
				w.Header().Add(k, v)
			}
		}
	}
	w.WriteHeader(statusFromError(err))
	_, _ = w.Write(body)
}

// JSONErrorEncoder encodes errors as simple JSON.
func JSONErrorEncoder(_ context.Context, err error, w http.ResponseWriter) {
	code := statusFromError(err)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":  err.Error(),
		"status": code,
	})
}
