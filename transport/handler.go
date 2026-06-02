package transport

import "context"

// Handler is implemented by application code that consumes events.
// Returning a non-nil error signals the transport to nack / retry the
// message according to its semantics.
type Handler[T any] interface {
	Handle(ctx context.Context, event T) error
}

// HandlerFunc adapts a plain function to the Handler interface.
type HandlerFunc[T any] func(ctx context.Context, event T) error

// Handle satisfies the Handler interface.
func (f HandlerFunc[T]) Handle(ctx context.Context, event T) error {
	return f(ctx, event)
}
