package ratelimit

import (
	"context"

	"google.golang.org/grpc"

	"github.com/fromforgesoftware/go-kit/auth"
	apierrors "github.com/fromforgesoftware/go-kit/errors"
)

// CtxKeyFunc derives the bucket key from a gRPC call's context.
type CtxKeyFunc func(context.Context) string

// OrgKey keys a call on the active organization claim.
func OrgKey(ctx context.Context) string {
	if org, ok := auth.OrgIDFromCtx(ctx); ok {
		return "org:" + org
	}
	return "anonymous"
}

// UnaryInterceptor enforces a Limiter on unary gRPC calls; a denied call
// returns a RateLimited error (mapped to ResourceExhausted by the kit).
func UnaryInterceptor(l Limiter, key CtxKeyFunc, policy Policy) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		res, err := l.Allow(ctx, key(ctx), policy)
		if err != nil {
			return nil, err
		}
		if !res.Allowed {
			return nil, apierrors.RateLimited("rate limit exceeded")
		}
		return handler(ctx, req)
	}
}
