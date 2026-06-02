package middleware

import (
	"context"
	"time"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"google.golang.org/grpc"
)

// Logging middleware logs all gRPC requests with method, duration, and error status
func Logging(log logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		// Call the handler
		resp, err := handler(ctx, req)

		// Log the request
		duration := time.Since(start)

		if err != nil {
			log.Error("gRPC request failed: method=%s duration=%v error=%v",
				info.FullMethod, duration, err)
		} else {
			log.Info("gRPC request completed: method=%s duration=%v",
				info.FullMethod, duration)
		}

		return resp, err
	}
}
