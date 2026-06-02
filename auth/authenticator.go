package auth

import (
	"context"
	"time"

	"github.com/fromforgesoftware/go-kit/auth/jwt"
	"github.com/fromforgesoftware/go-kit/errors"
	"github.com/fromforgesoftware/go-kit/firebase"
)

const (
	AuthorizationHeader = "Authorization"
	BearerPrefix        = "Bearer "
)

type baseAuthenticator[R any] struct {
	tokenExtractor  TokenExtractor[R]
	contextInjector ContextInjector
	firebaseClient  firebase.Client
	hmacValidator   jwt.Validator
}

func NewBaseAuthenticator[R any](
	tokenExtractor TokenExtractor[R],
	contextInjector ContextInjector,
	firebaseClient firebase.Client,
	hmacValidator jwt.Validator,
) *baseAuthenticator[R] {
	return &baseAuthenticator[R]{
		tokenExtractor:  tokenExtractor,
		contextInjector: contextInjector,
		firebaseClient:  firebaseClient,
		hmacValidator:   hmacValidator,
	}
}

func (a *baseAuthenticator[R]) ValidateToken(ctx context.Context, token Token) (Token, error) {
	switch token.Type() {
	case TokenTypeHMAC:
		return a.validateHmacToken(ctx, token.Value())
	case TokenTypeFirebase:
		return a.validateFirebaseToken(ctx, token.Value())
	default:
		// Fallback for backward compatibility or robust handling
		// Try Firebase first
		firebaseToken, err := a.validateFirebaseToken(ctx, token.Value())
		if err == nil {
			return firebaseToken, nil
		}
		// Then try HMAC
		return a.validateHmacToken(ctx, token.Value())
	}
}

func (a *baseAuthenticator[R]) validateFirebaseToken(ctx context.Context, tokenStr string) (Token, error) {
	if a.firebaseClient == nil {
		return nil, errors.Unauthorized("Firebase authentication not configured")
	}
	claims, err := a.firebaseClient.Auth().VerifyIDToken(ctx, tokenStr)
	if err != nil {
		return nil, err
	}

	adapter := &firebaseClaimsAdapter{claims: claims}
	return NewToken(tokenStr, TokenTypeFirebase, adapter)
}

func (a *baseAuthenticator[R]) validateHmacToken(ctx context.Context, tokenStr string) (Token, error) {
	if a.hmacValidator == nil {
		return nil, errors.Unauthorized("HMAC issuer not configured")
	}

	claims, err := a.hmacValidator.Validate(ctx, tokenStr)
	if err != nil {
		return nil, err
	}

	token, err := NewToken(tokenStr, TokenTypeHMAC, claims)
	if err != nil {
		return nil, err
	}
	return token, nil
}

type firebaseClaimsAdapter struct {
	claims firebase.TokenClaims
}

func (a *firebaseClaimsAdapter) Subject() string {
	return a.claims.ID()
}

func (a *firebaseClaimsAdapter) Expiry() time.Time {
	if exp, ok := a.claims.Claims()["exp"].(float64); ok {
		return time.Unix(int64(exp), 0)
	}
	return time.Time{}
}

func (a *firebaseClaimsAdapter) Get(key string) any {
	return a.claims.Claims()[key]
}
