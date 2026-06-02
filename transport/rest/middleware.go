package rest

import "net/http"

// Middleware wraps an http.Handler with cross-cutting behaviour
// (authentication, logging, recovery, tracing, ...).
type Middleware interface {
	Intercept(http.Handler) http.Handler
}

// MiddlewareFunc is the adapter that lets a plain function satisfy
// the Middleware interface.
type MiddlewareFunc func(http.Handler) http.Handler

func (f MiddlewareFunc) Intercept(h http.Handler) http.Handler {
	return f(h)
}
