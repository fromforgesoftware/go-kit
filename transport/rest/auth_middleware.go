package rest

import (
	"net/http"
)

type HTTPAuthenticator interface {
	Authenticate(req *http.Request) error
}

type AuthMiddleware struct {
	authenticator HTTPAuthenticator
	errorEncoder  ErrorEncoder
}

type authMiddlewareOption func(*AuthMiddleware)

func defaultAuthMiddlewareOpts() []authMiddlewareOption {
	return []authMiddlewareOption{
		WithErrorEncoder(JSONErrorEncoder),
	}
}

func WithErrorEncoder(encoder ErrorEncoder) authMiddlewareOption {
	return func(m *AuthMiddleware) {
		m.errorEncoder = encoder
	}
}

// NewAuthMiddleware creates a Middleware that runs the supplied
// authenticator against every incoming request. On authentication
// failure the configured errorEncoder writes the error response and
// the inner handler is not invoked.
//
// Wire it into a Controller's Routes via Router.Use:
//
//	r.Use(kitrest.NewAuthMiddleware(authenticator))
//
// or scope it to a single registration with With():
//
//	r.With(kitrest.NewAuthMiddleware(authenticator)).Post("/", h)
func NewAuthMiddleware(authenticator HTTPAuthenticator, opts ...authMiddlewareOption) *AuthMiddleware {
	middleware := &AuthMiddleware{
		authenticator: authenticator,
	}
	for _, opt := range append(defaultAuthMiddlewareOpts(), opts...) {
		opt(middleware)
	}
	return middleware
}

// Intercept implements the Middleware interface.
func (m *AuthMiddleware) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := m.authenticator.Authenticate(r); err != nil {
			m.errorEncoder(r.Context(), err, w)
			return
		}
		next.ServeHTTP(w, r)
	})
}
