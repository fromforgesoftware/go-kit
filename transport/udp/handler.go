package udp

import (
	"context"
	"fmt"
	"sync"

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
		Error("UDP Handler Error")
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

// HandlerConfig holds the configuration for the UDP handler.
type HandlerConfig struct {
	errorHandler ErrorHandler
}

// Handler handles a UDP packet
type Handler interface {
	Handle(ctx context.Context, session Session, payload []byte) error
}

// HandlerFunc adapts a function to the Handler interface
type HandlerFunc func(ctx context.Context, session Session, payload []byte) error

func (f HandlerFunc) Handle(ctx context.Context, session Session, payload []byte) error {
	return f(ctx, session, payload)
}

type (
	DecodeRequestFunc[Req any]  func(context.Context, []byte) (Req, error)
	EncodeResponseFunc[Res any] func(context.Context, Res) ([]byte, error)
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

// NewHandler creates a generic UDP handler.
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
			packet, err := enc(ctx, res)
			if err != nil {
				cfg.errorHandler.Handle(ctx, err)
				return err
			}

			// For UDP we usually send one response packet (or multiple if we handled fragmentation, but here we assume simple response)
			if len(packet) > 0 {
				if err := session.Send(packet); err != nil {
					cfg.errorHandler.Handle(ctx, err)
					return err
				}
			}
		}

		return nil
	})
}

// Registry allows registering handlers for opcodes. Matches tcp.Registry
// so a service's controller code is identical across the two transports.
type Registry interface {
	Register(opcode uint16, handler Handler)
	RegisterFunc(opcode uint16, handler func(context.Context, Session, []byte) error)
}

// Mux is a packet router
type Mux struct {
	handlers map[uint16]Handler
	mu       sync.RWMutex
}

// NewMux creates a new Mux
func NewMux() *Mux {
	return &Mux{
		handlers: make(map[uint16]Handler),
	}
}

// Register registers a handler for the given opcode
func (m *Mux) Register(opcode uint16, handler Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[opcode] = handler
}

// RegisterFunc registers a handler function for the given opcode.
func (m *Mux) RegisterFunc(opcode uint16, handler func(context.Context, Session, []byte) error) {
	m.Register(opcode, HandlerFunc(handler))
}

// Handle implements Handler
func (m *Mux) Handle(ctx context.Context, session Session, payload []byte) error {
	if len(payload) < 2 {
		return fmt.Errorf("payload too short for opcode")
	}

	// Read Opcode (Little Endian, first 2 bytes of payload)
	opcodeVal := uint16(payload[0]) | uint16(payload[1])<<8

	m.mu.RLock()
	handler, ok := m.handlers[opcodeVal]
	m.mu.RUnlock()

	if !ok {
		return nil
	}

	return handler.Handle(ctx, session, payload)
}
