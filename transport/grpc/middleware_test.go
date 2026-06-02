package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fromforgesoftware/go-kit/monitoring/logger/loggertest"
	transportgrpc "github.com/fromforgesoftware/go-kit/transport/grpc"
	"github.com/fromforgesoftware/go-kit/transport/grpc/middleware"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

func TestMiddlewareChaining(t *testing.T) {
	tests := []struct {
		name           string
		middlewares    []transportgrpc.Middleware
		handlerReturns error
		wantErr        bool
	}{
		{
			name: "chains recovery and logging middlewares together",
			middlewares: []transportgrpc.Middleware{
				transportgrpc.MiddlewareFunc(middleware.Recovery(loggertest.NewStubLogger(t))),
				transportgrpc.MiddlewareFunc(middleware.Logging(loggertest.NewStubLogger(t))),
			},
			handlerReturns: nil,
			wantErr:        false,
		},
		{
			name: "chains all three middlewares and propagates errors",
			middlewares: []transportgrpc.Middleware{
				transportgrpc.MiddlewareFunc(middleware.Recovery(loggertest.NewStubLogger(t))),
				transportgrpc.MiddlewareFunc(middleware.Logging(loggertest.NewStubLogger(t))),
				transportgrpc.MiddlewareFunc(middleware.RequestID()),
			},
			handlerReturns: errors.New("test error"),
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerFunc := func(ctx context.Context, req interface{}) (interface{}, error) {
				return "response", tt.handlerReturns
			}

			chained := transportgrpc.ChainUnaryServer(tt.middlewares...)
			ctx := context.Background()
			info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

			resp, err := chained(ctx, "request", info, handlerFunc)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "response", resp)
			}
		})
	}
}
