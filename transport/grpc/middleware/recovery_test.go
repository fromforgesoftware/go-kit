package middleware_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fromforgesoftware/go-kit/monitoring/logger/loggertest"
	"github.com/fromforgesoftware/go-kit/transport/grpc/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRecoveryMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		handlerFunc    grpc.UnaryHandler
		wantPanic      bool
		wantErrCode    codes.Code
		wantLogMessage string
	}{
		{
			name: "no panic returns successful response",
			handlerFunc: func(ctx context.Context, req interface{}) (interface{}, error) {
				return "success", nil
			},
			wantPanic:   false,
			wantErrCode: codes.OK,
		},
		{
			name: "panic with string converts to internal error",
			handlerFunc: func(ctx context.Context, req interface{}) (interface{}, error) {
				panic("something went wrong")
			},
			wantPanic:      true,
			wantErrCode:    codes.Internal,
			wantLogMessage: "panic recovered in gRPC handler",
		},
		{
			name: "panic with error converts to internal error",
			handlerFunc: func(ctx context.Context, req interface{}) (interface{}, error) {
				panic(errors.New("critical failure"))
			},
			wantPanic:      true,
			wantErrCode:    codes.Internal,
			wantLogMessage: "panic recovered in gRPC handler",
		},
		{
			name: "panic with nil converts to internal error",
			handlerFunc: func(ctx context.Context, req interface{}) (interface{}, error) {
				panic(nil)
			},
			wantPanic:      true,
			wantErrCode:    codes.Internal,
			wantLogMessage: "panic recovered in gRPC handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := loggertest.NewStubLogger(t)
			interceptor := middleware.Recovery(log)

			ctx := context.Background()
			req := "test request"
			info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

			resp, err := interceptor(ctx, req, info, tt.handlerFunc)

			if tt.wantPanic {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok, "error should be a gRPC status error")
				assert.Equal(t, tt.wantErrCode, st.Code())
				// Logger stub doesn't capture messages, just verify error was returned
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "success", resp)
			}
		})
	}
}
