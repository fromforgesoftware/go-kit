package middleware

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const requestIDKey = "x-request-id"

type requestIDCtxKey struct{}

// RequestID middleware extracts or generates a request ID and adds it to context
func RequestID() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Try to extract request ID from metadata
		var requestID string
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if ids := md.Get(requestIDKey); len(ids) > 0 {
				requestID = ids[0]
			}
		}

		// Generate new ID if not provided
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Add to context
		ctx = context.WithValue(ctx, requestIDCtxKey{}, requestID)

		// Add to outgoing metadata
		ctx = metadata.AppendToOutgoingContext(ctx, requestIDKey, requestID)

		return handler(ctx, req)
	}
}

// GetRequestID retrieves the request ID from context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDCtxKey{}).(string); ok {
		return id
	}
	return ""
}
