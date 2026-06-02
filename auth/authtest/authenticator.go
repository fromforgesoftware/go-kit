// Package authtest provides test fixtures (stub authenticators, signed
// JWTs, etc.) that downstream services use to exercise auth-protected
// HTTP / gRPC handlers without spinning up a real identity provider.
package authtest

import (
	"context"
	"net/http"

	"github.com/fromforgesoftware/go-kit/auth"
	"google.golang.org/grpc/metadata"
)

type httpAuthenticator struct {
}

func NewHTTPAuthenticator() *httpAuthenticator {
	return &httpAuthenticator{}
}

func (a *httpAuthenticator) Authenticate(r *http.Request) error {
	// Always return nil for testing purposes, ignore any token validation
	return nil
}

// grpcAuthenticator always authenticates the given subject.
// Use this in tests where you need a fixed authenticated user without
// parsing real auth headers.
type grpcAuthenticator struct {
	subject string
}

type grpcAuthenticatorOption func(*grpcAuthenticator)

func defaultOptions() []grpcAuthenticatorOption {
	return []grpcAuthenticatorOption{
		WithGrpcAuthenticatorSubject("test-user-123"),
	}
}

func WithGrpcAuthenticatorSubject(subject string) grpcAuthenticatorOption {
	return func(a *grpcAuthenticator) {
		a.subject = subject
	}
}

func NewGrpcAuthenticator(opts ...grpcAuthenticatorOption) *grpcAuthenticator {
	authenticator := &grpcAuthenticator{}
	for _, option := range append(defaultOptions(), opts...) {
		option(authenticator)
	}
	return authenticator
}

func (a *grpcAuthenticator) Authenticate(ctx context.Context, md metadata.MD) (context.Context, error) {
	// Create a token with the fixed subject
	token, _ := auth.NewToken(
		"mock-token",
		auth.TokenTypeJWT,
		NewTokenClaims(a.subject),
	)
	return auth.InjectTokenInCtx(ctx, token), nil
}
