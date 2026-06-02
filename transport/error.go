package transport

import "context"

type ErrorHandler interface {
	Handle(ctx context.Context, err error)
}
