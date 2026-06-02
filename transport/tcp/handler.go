package tcp

import (
	"context"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/transport"
)

// ErrorHandler handles errors processed by the Handler.
type ErrorHandler interface {
	Handle(ctx context.Context, err error)
}

// LogErrorHandler is a basic ErrorHandler that logs errors.
type LogErrorHandler struct {
	logger logger.Logger
}

func NewLogErrorHandler(logger logger.Logger) *LogErrorHandler {
	return &LogErrorHandler{logger: logger}
}

func (h *LogErrorHandler) Handle(ctx context.Context, err error) {
	h.logger.
		WithContext(ctx).
		WithKeysAndValues("error", err).
		Error("TCP Handler Error")
}

// HandlerOption sets an optional parameter for the Handler.
type HandlerOption func(*HandlerConfig)

func defaultHandlerErrorHandler() []HandlerOption {
	return []HandlerOption{
		HandlerErrorHandler(NewLogErrorHandler(logger.New())),
	}
}

// HandlerErrorHandler sets the error handler for the server.
func HandlerErrorHandler(h ErrorHandler) HandlerOption {
	return func(c *HandlerConfig) {
		c.errorHandler = h
	}
}

// HandlerConfig holds the configuration for the TCP handler.
type HandlerConfig struct {
	errorHandler ErrorHandler
}

// Handler handles a TCP session and payload
type Handler interface {
	Handle(context.Context, Session, []byte) error
}

// Middleware is a function that wraps a Handler
type Middleware func(Handler) Handler

// HandlerFunc allows a function to be used as a Handler
type HandlerFunc func(ctx context.Context, session Session, payload []byte) error

func (f HandlerFunc) Handle(ctx context.Context, session Session, payload []byte) error {
	return f(ctx, session, payload)
}

type (
	DecodeRequestFunc[Req any]  func(context.Context, []byte) (Req, error)
	EncodeResponseFunc[Res any] func(context.Context, Res) ([][]byte, error)
)

type sessionKey struct{}

// SessionFromContext retrieves the session from the context
func SessionFromContext(ctx context.Context) Session {
	val := ctx.Value(sessionKey{})
	if s, ok := val.(Session); ok {
		return s
	}
	return nil
}

// NewContextWithSession creates a new context with the given session attached
func NewContextWithSession(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, session)
}

// NewHandler creates a generic TCP handler.
// It decodes the payload, calls the endpoint, and encodes the response using the EncodeResponseFunc.
// The encoder MUST return [][]byte, where each slice is a separate packet.
// If EncodeResponseFunc is nil, it assumes no response is sent back.
func NewHandler[Req, Res any](
	e transport.Endpoint[Req, Res],
	dec DecodeRequestFunc[Req],
	enc EncodeResponseFunc[Res],
	options ...HandlerOption,
) Handler {
	// Default options
	cfg := &HandlerConfig{}

	for _, option := range append(defaultHandlerErrorHandler(), options...) {
		option(cfg)
	}

	return HandlerFunc(func(ctx context.Context, session Session, payload []byte) error {
		// Inject session into context
		ctx = context.WithValue(ctx, sessionKey{}, session)

		req, err := dec(ctx, payload)
		if err != nil {
			cfg.errorHandler.Handle(ctx, err)
			return err
		}

		res, err := e(ctx, req)
		if err != nil {
			cfg.errorHandler.Handle(ctx, err)
			return err
		}

		if enc != nil {
			packets, err := enc(ctx, res)
			if err != nil {
				cfg.errorHandler.Handle(ctx, err)
				return err
			}

			for _, p := range packets {
				if err := session.Send(p); err != nil {
					cfg.errorHandler.Handle(ctx, err)
					return err
				}
			}
		}

		return nil
	})
}
