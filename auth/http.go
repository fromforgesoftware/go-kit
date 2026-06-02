package auth

import (
	"net/http"

	"github.com/fromforgesoftware/go-kit/auth/jwt"
	"github.com/fromforgesoftware/go-kit/firebase"
)

type httpAuthenticator struct {
	baseAuthenticator[*http.Request]
}

func NewHttpAuthenticator(
	tokenExtractor TokenExtractor[*http.Request],
	contextInjector ContextInjector,
	firebaseClient firebase.Client,
	hmacValidator jwt.Validator,
) *httpAuthenticator {
	return &httpAuthenticator{
		baseAuthenticator: *NewBaseAuthenticator(tokenExtractor, contextInjector, firebaseClient, hmacValidator),
	}
}

func (a *httpAuthenticator) Authenticate(r *http.Request) error {
	token, err := a.tokenExtractor.Extract(r.Context(), r)
	if err != nil {
		return err
	}

	verifiedToken, err := a.ValidateToken(r.Context(), token)
	if err != nil {
		return err
	}

	ctx, err := a.contextInjector.Inject(r.Context(), verifiedToken)
	if err != nil {
		return err
	}

	*r = *r.WithContext(ctx)

	return nil
}
