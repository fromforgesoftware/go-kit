package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	kiterrors "github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/transport/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statusCoderErr is a minimal legacy-style error that implements the
// StatusCoder interface (the surface DefaultErrorEncoder/JSONErrorEncoder
// have always honoured).
type statusCoderErr struct {
	status int
	msg    string
}

func (e statusCoderErr) Error() string   { return e.msg }
func (e statusCoderErr) StatusCode() int { return e.status }

func TestJSONErrorEncoder(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "kit-errors Unauthenticated → 401",
			err:        kiterrors.Unauthenticated("Authorization header is required"),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "kit-errors NotFound → 404",
			err:        kiterrors.NotFound("user", "abc"),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "kit-errors Conflict → 409",
			err:        kiterrors.Conflict("name taken"),
			wantStatus: http.StatusConflict,
		},
		{
			name:       "kit-errors MissingField → 400",
			err:        kiterrors.MissingField("name"),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "kit-errors wrapped via fmt.Errorf still resolves",
			err:        fmt.Errorf("ctx: %w", kiterrors.Unauthenticated("nope")),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "legacy StatusCoder honoured",
			err:        statusCoderErr{status: http.StatusTeapot, msg: "I'm a teapot"},
			wantStatus: http.StatusTeapot,
		},
		{
			name:       "plain error falls back to 500",
			err:        errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			rest.JSONErrorEncoder(context.Background(), tc.err, rec)
			assert.Equal(t, tc.wantStatus, rec.Code)
			assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))

			var body map[string]interface{}
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
			assert.Equal(t, float64(tc.wantStatus), body["status"])
			assert.NotEmpty(t, body["error"])
		})
	}
}

func TestDefaultErrorEncoderUsesKitHTTPStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rest.DefaultErrorEncoder(context.Background(),
		kiterrors.Unauthenticated("Authorization header is required"), rec)
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"DefaultErrorEncoder must read HTTPStatus() off kit errors, not fall back to 500")
}

func TestJsonApiErrorEncoder(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"Unauthenticated → 401", kiterrors.Unauthenticated("nope"), http.StatusUnauthorized},
		{"Conflict → 409", kiterrors.Conflict("name taken"), http.StatusConflict},
		{"plain error falls back to 500", errors.New("boom"), http.StatusInternalServerError},
		// Regression — apierrors.New(code, ...) without WithHTTPStatus(...)
		// reports HTTPStatus() == 0. Before the guard the encoder fed 0
		// to WriteHeader and panicked.
		{"kit error with HTTPStatus()=0 falls back to 500", kiterrors.New(kiterrors.CodeInternalError, kiterrors.WithMessage("no status set")), http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			require.NotPanics(t, func() {
				rest.JsonApiErrorEncoder(context.Background(), tc.err, rec)
			})
			assert.Equal(t, tc.wantStatus, rec.Code)
			assert.Equal(t, "application/vnd.api+json", rec.Header().Get("Content-Type"))

			// The JSON:API error object's `status` field must mirror the
			// wire status — otherwise a client that trusts the body
			// disagrees with what HTTP just said.
			var doc struct {
				Errors []struct {
					Status string `json:"status"`
				} `json:"errors"`
			}
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&doc))
			require.NotEmpty(t, doc.Errors, "expected at least one error object in body")
			for _, e := range doc.Errors {
				assert.Equal(t, strconv.Itoa(tc.wantStatus), e.Status,
					"body status must match the HTTP status")
			}
		})
	}
}
