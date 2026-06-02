package auth

import (
	"context"

	"github.com/fromforgesoftware/go-kit/auth/jwt"
	"github.com/fromforgesoftware/go-kit/firebase"
	"go.uber.org/fx"
	"google.golang.org/grpc/metadata"
)

type grpcAuthenticator struct {
	baseAuthenticator[metadata.MD]
}

type GrpcAuthenticatorParams struct {
	fx.In

	TokenExtractor  TokenExtractor[metadata.MD]
	ContextInjector ContextInjector
	FirebaseClient  firebase.Client `optional:"true"`
	HmacValidator   jwt.Validator
}

func NewGrpcAuthenticator(params GrpcAuthenticatorParams) *grpcAuthenticator {
	return &grpcAuthenticator{
		baseAuthenticator: *NewBaseAuthenticator(
			params.TokenExtractor,
			params.ContextInjector,
			params.FirebaseClient,
			params.HmacValidator,
		),
	}
}

func (a *grpcAuthenticator) Authenticate(ctx context.Context, md metadata.MD) (context.Context, error) {
	token, err := a.tokenExtractor.Extract(ctx, md)
	if err != nil {
		return ctx, err
	}

	verifiedToken, err := a.ValidateToken(ctx, token)
	if err != nil {
		return ctx, err
	}

	ctx, err = a.contextInjector.Inject(ctx, verifiedToken)
	if err != nil {
		return ctx, err
	}
	return ctx, nil
}
