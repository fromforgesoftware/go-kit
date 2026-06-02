package auth

import "context"

type ContextInjector interface {
	Inject(ctx context.Context, token Token) (context.Context, error)
}

type contextKeyType int

const (
	tokenCtxKey contextKeyType = iota
)

func TokenFromCtx(ctx context.Context) Token {
	token := ctx.Value(tokenCtxKey)
	if token == nil {
		return nil
	}

	return token.(Token)
}

func InjectTokenInCtx(ctx context.Context, token Token) context.Context {
	return context.WithValue(ctx, tokenCtxKey, token)
}

type tokenContextInjector struct{}

func NewTokenContextInjector() *tokenContextInjector {
	return &tokenContextInjector{}
}

func (a *tokenContextInjector) Inject(ctx context.Context, token Token) (context.Context, error) {
	return InjectTokenInCtx(ctx, token), nil
}
