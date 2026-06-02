package authtest

import (
	"context"

	"github.com/fromforgesoftware/go-kit/auth"
	"github.com/fromforgesoftware/go-kit/errors"
	"google.golang.org/grpc/metadata"
)

// strictGrpcAuthenticator validates that proper authorization headers are present
// Use this in tests that need to verify auth middleware behavior
type strictGrpcAuthenticator struct {
	subject string
}

// NewStrictGrpcAuthenticator creates an authenticator that actually validates auth headers
func NewStrictGrpcAuthenticator(subject string) *strictGrpcAuthenticator {
	return &strictGrpcAuthenticator{subject: subject}
}

func (a *strictGrpcAuthenticator) Authenticate(ctx context.Context, md metadata.MD) (context.Context, error) {
	// Check for authorization header
	authHeaders := md.Get(auth.AuthorizationHeader)
	if len(authHeaders) == 0 {
		return ctx, errors.Unauthorized("missing authorization header")
	}

	// For test purposes, just check header exists and create token
	token, _ := auth.NewToken(
		"mock-token",
		auth.TokenTypeJWT,
		NewTokenClaims(a.subject),
	)
	return auth.InjectTokenInCtx(ctx, token), nil
}
