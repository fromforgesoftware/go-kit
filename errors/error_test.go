package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrorIs(t *testing.T) {
	t.Run("errors with same code should be equal", func(t *testing.T) {
		err1 := NotFound("user", "123")
		err2 := NotFound("account", "456")

		// Both are NotFound errors, so they should be considered equal by errors.Is
		assert.True(t, errors.Is(err1, err2))
	})

	t.Run("errors with different codes should not be equal", func(t *testing.T) {
		err1 := NotFound("user", "123")
		err2 := Conflict("conflict message")

		// Different error codes, should not be equal
		assert.False(t, errors.Is(err1, err2))
	})

	t.Run("custom error should work with standard errors.Is", func(t *testing.T) {
		notFoundErr := NotFound("user", "123")
		standardErr := errors.New("standard error")

		// Should not be equal to standard error
		assert.False(t, errors.Is(notFoundErr, standardErr))
		assert.False(t, errors.Is(standardErr, notFoundErr))
	})
}

func TestWrap(t *testing.T) {
	origErr := fmt.Errorf("original error")

	t.Run("Wrap passes cause", func(t *testing.T) {
		err := Wrap(origErr, CodeInternalError, WithMessage("wrapped"))

		require.NotNil(t, err)
		assert.Equal(t, CodeInternalError, err.Code())
		assert.Equal(t, "wrapped", err.Message())
		assert.Equal(t, origErr, err.Cause())

		// Verify Unwrap interface implementation
		u, ok := err.(interface{ Unwrap() error })
		require.True(t, ok)
		assert.Equal(t, origErr, u.Unwrap())
	})

	t.Run("Wrap maintains options", func(t *testing.T) {
		err := Wrap(origErr, CodeInternalError, WithHTTPStatus(500), WithRequestID("req-123"))

		assert.Equal(t, 500, err.HTTPStatus())
		assert.Equal(t, "req-123", err.RequestID())
	})
}

func TestJSONMarshaling(t *testing.T) {
	t.Run("Marshal and Unmarshal", func(t *testing.T) {
		now := time.Now().Truncate(time.Second) // Truncate for JSON precision comparison

		detail := NewDetail("field1", CodeInvalidFormat, "invalid", "val")
		origErr := New(CodeValidationFailed,
			WithMessage("validation failed"),
			WithDetails(detail),
			WithRequestID("req-1"),
			WithService("svc-1"),
			WithHTTPStatus(400),
		)

		// Hack to set timestamp for deterministic test
		origErr.(*errorImpl).timestamp = now

		data, err := json.Marshal(origErr)
		require.NoError(t, err)

		var unmarshaledErr errorImpl
		err = json.Unmarshal(data, &unmarshaledErr)
		require.NoError(t, err)

		assert.Equal(t, origErr.Code(), unmarshaledErr.Code())
		assert.Equal(t, origErr.Message(), unmarshaledErr.Message())
		assert.Equal(t, origErr.RequestID(), unmarshaledErr.RequestID())
		assert.Equal(t, origErr.Service(), unmarshaledErr.Service())
		assert.Equal(t, origErr.HTTPStatus(), unmarshaledErr.HTTPStatus())

		// Check details
		require.Len(t, unmarshaledErr.Details(), 1)
		d := unmarshaledErr.Details()[0]
		assert.Equal(t, detail.Field(), d.Field())
		assert.Equal(t, detail.Code(), d.Code())
		assert.Equal(t, detail.Message(), d.Message())
		assert.Equal(t, detail.Value(), d.Value())

		// Check timestamp
		assert.True(t, now.Equal(unmarshaledErr.Timestamp()) || now.Sub(unmarshaledErr.Timestamp()) < time.Second, "Timestamp mismatch")
	})
}

func TestGRPCMapping(t *testing.T) {
	t.Run("FromGRPCError handles standard codes", func(t *testing.T) {
		tests := []struct {
			grpcCode     codes.Code
			expectedCode Code
		}{
			{codes.NotFound, CodeNotFound},
			{codes.InvalidArgument, CodeInvalidArgument},
			{codes.AlreadyExists, CodeAlreadyExists},
			{codes.PermissionDenied, CodeForbidden},
			{codes.Unauthenticated, CodeUnauthenticated},
			{codes.Internal, CodeInternalError},
			{codes.Unknown, CodeInternalError},
		}

		for _, tt := range tests {
			st := status.New(tt.grpcCode, "some error")
			err := st.Err()

			mappedErr := FromGRPCError(err)

			// It should be our Error type
			ourErr, ok := As(mappedErr)
			require.True(t, ok)
			assert.Equal(t, tt.expectedCode, ourErr.Code())
			assert.Equal(t, "some error", ourErr.Message())
		}
	})

	t.Run("FromGRPCError bypasses non-grpc errors", func(t *testing.T) {
		stdErr := fmt.Errorf("std error")
		res := FromGRPCError(stdErr)
		assert.Equal(t, stdErr, res)
	})

	t.Run("FromGRPCError handles nil", func(t *testing.T) {
		res := FromGRPCError(nil)
		assert.Nil(t, res)
	})
}

func TestMetadata(t *testing.T) {
	t.Run("WithService", func(t *testing.T) {
		err := New(CodeInternalError, WithService("pay-service"))
		assert.Equal(t, "pay-service", err.Service())
	})

	t.Run("WithRequestID", func(t *testing.T) {
		err := New(CodeInternalError, WithRequestID("12345"))
		assert.Equal(t, "12345", err.RequestID())
	})
}
