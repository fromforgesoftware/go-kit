package grpc

type HandlerMiddlewareFunc func(Handler) Handler

func (f HandlerMiddlewareFunc) Intercept(h Handler) Handler {
	return f(h)
}

// HandlerMiddleware wraps a Handler with cross-cutting behaviour (auth,
// logging, metrics, etc.) before delegating to the next link in the chain.
type HandlerMiddleware interface {
	Intercept(Handler) Handler
}

func chain(handler Handler, middlewares ...HandlerMiddleware) Handler {
	h := handler
	for _, m := range middlewares {
		h = m.Intercept(h)
	}
	return h
}
