// Package middleware provides common gRPC server middlewares
package middleware

import (
	"context"
	"runtime/debug"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Recovery middleware recovers from panics and returns Internal error
func Recovery(log logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				// Log the panic with stack trace
				log.Error("panic recovered in gRPC handler: %v\nmethod: %s\nstack: %s",
					r, info.FullMethod, string(debug.Stack()))

				// Return Internal error to client
				err = status.Errorf(codes.Internal, "internal server error: %v", r)
			}
		}()

		return handler(ctx, req)
	}
}
