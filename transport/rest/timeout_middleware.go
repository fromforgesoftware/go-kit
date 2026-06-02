package rest

import (
	"context"
	"net/http"
	"time"
)

// TimeoutMiddleware enforces a per-request deadline. When the handler
// hasn't finished by the time limit, the request context is cancelled
// — handlers respecting ctx.Done() will return, and the client sees
// 503 Service Unavailable.
type TimeoutMiddleware struct {
	timeout time.Duration
	message string
}

type timeoutOption func(*TimeoutMiddleware)

// WithTimeoutMessage overrides the body returned when the deadline fires.
func WithTimeoutMessage(msg string) timeoutOption {
	return func(m *TimeoutMiddleware) { m.message = msg }
}

func NewTimeoutMiddleware(timeout time.Duration, opts ...timeoutOption) *TimeoutMiddleware {
	m := &TimeoutMiddleware{timeout: timeout, message: "request timed out"}
	for _, o := range opts {
		o(m)
	}
	return m
}

func (m *TimeoutMiddleware) Intercept(next http.Handler) http.Handler {
	return http.TimeoutHandler(next, m.timeout, m.message)
}

// ContextTimeoutMiddleware is a lighter alternative: it cancels the request
// context after the timeout but does NOT block the response writer. Handlers
// that respect ctx.Done() will return early; others run to completion.
// Use this when you want timeouts to control downstream calls (DB, RPC) but
// don't want a blocked response writer.
type ContextTimeoutMiddleware struct {
	timeout time.Duration
}

func NewContextTimeoutMiddleware(timeout time.Duration) *ContextTimeoutMiddleware {
	return &ContextTimeoutMiddleware{timeout: timeout}
}

func (m *ContextTimeoutMiddleware) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), m.timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
