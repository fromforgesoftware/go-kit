package grpc

import (
	"context"

	"github.com/fromforgesoftware/go-kit/monitoring"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/transport"
)

type (
	Handler interface {
		ServeGRPC(ctx context.Context, request interface{}) (interface{}, error)
	}

	HandlerFunc func(ctx context.Context, request interface{}) (interface{}, error)
)

func (f HandlerFunc) ServeGRPC(ctx context.Context, request interface{}) (interface{}, error) {
	return f(ctx, request)
}

func Serve[I, O any](ctx context.Context, h Handler, in I) (O, error) {
	response, err := h.ServeGRPC(ctx, in)
	if err != nil {
		var result O
		return result, err
	}
	return response.(O), nil
}

func ServeEmptyRes[I any](ctx context.Context, h Handler, in I) error {
	_, err := h.ServeGRPC(ctx, in)
	return err
}

type (
	DecodeRequestFunc[I, O any]  func(context.Context, I) (request O, err error)
	EncodeResponseFunc[I, O any] func(context.Context, I) (response O, err error)
)

type (
	HandlerOpt    func(c *handlerConfig)
	handlerConfig struct {
		errorHandler  transport.ErrorHandler
		authenticator GRPCAuthenticator
		middlewares   []HandlerMiddleware
	}
)

type LogErrorHandler struct {
	logger logger.Logger
}

func NewLogErrorHandler(logger logger.Logger) *LogErrorHandler {
	return &LogErrorHandler{
		logger: logger,
	}
}

func (h *LogErrorHandler) Handle(ctx context.Context, err error) {
	h.logger.Error("gRPC error", err)
}

func defaultHandlerOpts(log logger.Logger) []HandlerOpt {
	return []HandlerOpt{
		WithHandlerErrorHandler(NewLogErrorHandler(log)),
	}
}

// WithHandlerErrorHandler sets a custom error handler for the gRPC handler
func WithHandlerErrorHandler(eh transport.ErrorHandler) HandlerOpt {
	return func(c *handlerConfig) {
		c.errorHandler = eh
	}
}

// WithAuthentication adds authentication middleware to the handler
// This is a cleaner alternative to wrapping with RequireAuthentication
func WithAuthentication(authenticator GRPCAuthenticator, opts ...authMiddlewareOption) HandlerOpt {
	return func(c *handlerConfig) {
		c.authenticator = authenticator
		c.middlewares = append(c.middlewares, NewAuthMiddleware(authenticator, opts...))
	}
}

// WithHandlerMiddlewares adds custom handler middlewares
func WithHandlerMiddlewares(middlewares ...HandlerMiddleware) HandlerOpt {
	return func(c *handlerConfig) {
		c.middlewares = append(c.middlewares, middlewares...)
	}
}

func NewHandler[DI, DO, EI, EO any](
	e transport.Endpoint[DO, EI],
	reqDecoder DecodeRequestFunc[DI, DO],
	reqEncoder EncodeResponseFunc[EI, EO],
	monitoring monitoring.Monitor,
	opts ...HandlerOpt,
) Handler {
	c := new(handlerConfig)
	for _, opt := range append(defaultHandlerOpts(monitoring.Logger()), opts...) {
		opt(c)
	}

	handler := HandlerFunc(func(ctx context.Context, req interface{}) (interface{}, error) {
		request, err := reqDecoder(ctx, req.(DI))
		if err != nil {
			c.errorHandler.Handle(ctx, err)
			return nil, err
		}

		response, err := e(ctx, request)
		if err != nil {
			c.errorHandler.Handle(ctx, err)
			return nil, err
		}

		grpcResp, err := reqEncoder(ctx, response)
		if err != nil {
			c.errorHandler.Handle(ctx, err)
			return nil, err
		}

		return grpcResp, nil
	})

	// Apply middlewares (e.g., authentication)
	return chain(handler, c.middlewares...)
}

func NewEmptyResHandler[DI, DO any](
	e transport.EmptyResEndpoint[DO],
	reqDecoder DecodeRequestFunc[DI, DO],
	monitor monitoring.Monitor,
	opts ...HandlerOpt,
) Handler {
	c := new(handlerConfig)
	for _, opt := range append(defaultHandlerOpts(monitor.Logger()), opts...) {
		opt(c)
	}

	handler := HandlerFunc(func(ctx context.Context, req interface{}) (interface{}, error) {
		request, err := reqDecoder(ctx, req.(DI))
		if err != nil {
			c.errorHandler.Handle(ctx, err)
			return nil, err
		}

		err = e(ctx, request)
		if err != nil {
			c.errorHandler.Handle(ctx, err)
			return nil, err
		}

		return nil, nil
	})

	// Apply middlewares (e.g., authentication)
	return chain(handler, c.middlewares...)
}
