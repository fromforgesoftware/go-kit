package grpc

import (
	"context"

	"google.golang.org/grpc"
)

// Middleware defines a gRPC unary server interceptor
type Middleware interface {
	Intercept(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error)
}

// MiddlewareFunc is an adapter to allow ordinary functions to be used as Middleware
type MiddlewareFunc grpc.UnaryServerInterceptor

// Intercept implements the Middleware interface
func (f MiddlewareFunc) Intercept(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	return f(ctx, req, info, handler)
}

// ChainUnaryServer creates a single interceptor out of a chain of many middlewares.
// Execution is done in left-to-right order, including passing of context.
// For example ChainUnaryServer(one, two, three) will execute one before two before three.
func ChainUnaryServer(middlewares ...Middleware) grpc.UnaryServerInterceptor {
	n := len(middlewares)

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		chainer := func(currentMiddleware Middleware, currentHandler grpc.UnaryHandler) grpc.UnaryHandler {
			return func(currentCtx context.Context, currentReq interface{}) (interface{}, error) {
				return currentMiddleware.Intercept(currentCtx, currentReq, info, currentHandler)
			}
		}

		chainedHandler := handler
		for i := n - 1; i >= 0; i-- {
			chainedHandler = chainer(middlewares[i], chainedHandler)
		}

		return chainedHandler(ctx, req)
	}
}
