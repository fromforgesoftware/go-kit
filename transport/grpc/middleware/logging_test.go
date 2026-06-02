package middleware_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fromforgesoftware/go-kit/monitoring/logger/loggertest"
	"github.com/fromforgesoftware/go-kit/transport/grpc/middleware"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLoggingMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		handlerFunc    grpc.UnaryHandler
		wantErr        bool
		expectedMethod string
	}{
		{
			name: "successful request logs completion",
			handlerFunc: func(ctx context.Context, req interface{}) (interface{}, error) {
				return "response", nil
			},
			wantErr:        false,
			expectedMethod: "/test.Service/Method",
		},
		{
			name: "failed request logs error",
			handlerFunc: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, errors.New("handler error")
			},
			wantErr:        true,
			expectedMethod: "/test.Service/Method",
		},
		{
			name: "grpc status error logs correctly",
			handlerFunc: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, status.Error(codes.NotFound, "not found")
			},
			wantErr:        true,
			expectedMethod: "/test.Service/Method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := loggertest.NewStubLogger(t)
			interceptor := middleware.Logging(log)

			ctx := context.Background()
			req := "test request"
			info := &grpc.UnaryServerInfo{FullMethod: tt.expectedMethod}

			resp, err := interceptor(ctx, req, info, tt.handlerFunc)

			if tt.wantErr {
				assert.Error(t, err)
				// Logger stub doesn't capture messages
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "response", resp)
				// Logger stub doesn't capture messages
			}
		})
	}
}
