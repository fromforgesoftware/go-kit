package middleware_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/transport/grpc/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRequestIDMiddleware(t *testing.T) {
	tests := []struct {
		name                string
		incomingRequestID   string
		wantRequestIDExists bool
	}{
		{
			name:                "generates new request id when none provided",
			incomingRequestID:   "",
			wantRequestIDExists: true,
		},
		{
			name:                "preserves existing request id from metadata",
			incomingRequestID:   "existing-request-id-123",
			wantRequestIDExists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedRequestID string
			handlerFunc := func(ctx context.Context, req interface{}) (interface{}, error) {
				capturedRequestID = middleware.GetRequestID(ctx)
				return "success", nil
			}

			interceptor := middleware.RequestID()
			ctx := context.Background()

			// Add incoming request ID if provided
			if tt.incomingRequestID != "" {
				md := metadata.New(map[string]string{"x-request-id": tt.incomingRequestID})
				ctx = metadata.NewIncomingContext(ctx, md)
			}

			req := "test request"
			info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

			resp, err := interceptor(ctx, req, info, handlerFunc)

			require.NoError(t, err)
			assert.Equal(t, "success", resp)

			if tt.wantRequestIDExists {
				assert.NotEmpty(t, capturedRequestID)
				if tt.incomingRequestID != "" {
					assert.Equal(t, tt.incomingRequestID, capturedRequestID)
				}
			}
		})
	}
}
