package udp

// Middleware wraps a Handler
type Middleware func(Handler) Handler

// Chain creates a single handler out of a chain of many middlewares.
// Execution is done in left-to-right order.
func Chain(middlewares ...Middleware) Middleware {
	return func(h Handler) Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			h = middlewares[i](h)
		}
		return h
	}
}

// MiddlewareFunc adapts a function to the Middleware interface
// Note: UDP Middleware is just func(Handler) Handler, so this might be redundant unless we want a specific interface.
// udp.Middleware provided in handler.go was `type Middleware func(Handler) Handler`.
