package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/fromforgesoftware/go-kit/auth"
)

// Authenticator is the contract every gRPC authentication middleware
// targets. The package previously exported this as `GRPCAuthenticator`
// which stutters as `grpc.GRPCAuthenticator`; the deprecated alias is
// kept for compatibility but new code should use Authenticator.
type Authenticator interface {
	Authenticate(ctx context.Context, md metadata.MD) (context.Context, error)
}

// GRPCAuthenticator is deprecated: use Authenticator. Kept as an alias
// to avoid breaking callers; remove in a follow-up release.
type GRPCAuthenticator = Authenticator

// authMiddleware struct for gRPC authentication middleware
type authMiddleware struct {
	authenticator GRPCAuthenticator
	errorHandler  func(error) error // Convert auth errors to gRPC errors
}

type authMiddlewareOption func(*authMiddleware)

// WithAuthErrorHandler sets a custom error handler for authentication errors
func WithAuthErrorHandler(handler func(error) error) authMiddlewareOption {
	return func(m *authMiddleware) {
		m.errorHandler = handler
	}
}

// defaultErrorHandler converts auth errors to gRPC status errors so the
// client sees codes.Unauthenticated instead of codes.Unknown. If the error
// already carries a gRPC status (i.e. an upstream interceptor mapped it),
// preserve it.
func defaultErrorHandler(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	return status.Error(codes.Unauthenticated, err.Error())
}

// NewAuthMiddleware creates a new gRPC authentication middleware
// Used internally by WithAuthentication option
func NewAuthMiddleware(authenticator GRPCAuthenticator, opts ...authMiddlewareOption) *authMiddleware {
	middleware := &authMiddleware{
		authenticator: authenticator,
		errorHandler:  defaultErrorHandler,
	}
	for _, opt := range opts {
		opt(middleware)
	}
	return middleware
}

// Intercept implements the (deprecated) HandlerMiddleware interface. New
// code should use AuthInterceptor and register it on the server chain via
// WithMiddlewares so panics in decode + auth flow through the same
// recovery/logging path as the rest of the chain.
func (m *authMiddleware) Intercept(next Handler) Handler {
	return HandlerFunc(func(ctx context.Context, req interface{}) (interface{}, error) {
		// Extract metadata from incoming context
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(map[string]string{})
		}

		// Authenticate the request
		newCtx, err := m.authenticator.Authenticate(ctx, md)
		if err != nil {
			return nil, m.errorHandler(err)
		}

		// Call the handler with the authenticated context
		return next.ServeGRPC(newCtx, req)
	})
}

// AuthInterceptor returns a regular grpc.UnaryServerInterceptor that
// performs authentication. Preferred over the per-handler
// WithAuthentication HandlerOpt because:
//   - It runs at the server chain, so recovery/logging/tracing wrap it.
//   - It receives the full grpc.UnaryServerInfo (method name) which the
//     handler-level wrapper cannot access.
//   - It uses the same Middleware / interceptor model as every other
//     server middleware, eliminating the dual abstraction.
//
// Wire it via grpc.WithMiddlewares(grpc.MiddlewareFunc(AuthInterceptor(...))).
func AuthInterceptor(authenticator Authenticator, opts ...authMiddlewareOption) grpc.UnaryServerInterceptor {
	m := NewAuthMiddleware(authenticator, opts...)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(map[string]string{})
		}
		newCtx, err := m.authenticator.Authenticate(ctx, md)
		if err != nil {
			return nil, m.errorHandler(err)
		}
		return handler(newCtx, req)
	}
}

// ============================================================================
// Client Authentication
// ============================================================================

// ClientAuthMiddleware creates a middleware that extracts auth token from context and adds it to metadata
func ClientAuthMiddleware() ClientMiddleware {
	return func(ctx context.Context) context.Context {
		// Get existing metadata or create new
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(map[string]string{})
		} else {
			md = md.Copy()
		}

		token := auth.TokenFromCtx(ctx)
		if token != nil {
			md.Set(auth.AuthorizationHeader, auth.BearerPrefix+token.Value())
		}

		return metadata.NewOutgoingContext(ctx, md)
	}
}
