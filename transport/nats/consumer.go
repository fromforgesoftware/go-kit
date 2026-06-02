package nats

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
	"github.com/fromforgesoftware/go-kit/transport"
)

const defaultConsumerTimeout = 30 * time.Second

type (
	// Decoder is NATS-specific because msg.Header / msg.Reply carry
	// broker metadata the transport.Decoder shape (raw []byte) cannot
	// express. Use a wrapper if you need broker-agnostic decoders.
	Decoder[T any] func(ctx context.Context, msg *nats.Msg) (T, error)

	// Handler and HandlerFunc are aliases over kit/transport so a
	// broker-agnostic handler fits any messaging transport.
	Handler[T any]     = transport.Handler[T]
	HandlerFunc[T any] = transport.HandlerFunc[T]

	Consumer interface {
		Subscribe(ctx context.Context) error
		Unsubscribe(ctx context.Context) error
	}

	// OnError is invoked for decode + handle failures so callers can
	// emit metrics / surface failures without owning the dispatch loop.
	OnError func(ctx context.Context, err error)

	consumer[T any] struct {
		conn    Connection
		log     logger.Logger
		decoder Decoder[T]
		handler Handler[T]
		config  consumerConfig
		sub     *nats.Subscription
		js      nats.JetStreamContext

		// Subscribe stores its ctx here so the per-message goroutine inherits
		// cancellation and the configured timeout — the previous code
		// hard-coded context.Background(), so handlers couldn't be cancelled
		// on shutdown.
		baseCtx    context.Context
		baseCtxMu  sync.RWMutex
		baseCancel context.CancelFunc
	}
)

func defaultConsumerOpts() []consumerOption {
	return []consumerOption{
		WithQueueGroup(""),
		WithConsumerTimeout(defaultConsumerTimeout),
		ConsumerWithTracer(noopTracer),
	}
}

func NewConsumer[T any](
	conn Connection,
	log logger.Logger,
	subject string,
	dec Decoder[T],
	handler Handler[T],
	opts ...consumerOption,
) (Consumer, error) {
	cfg := &consumerConfig{subject: subject}
	for _, opt := range append(defaultConsumerOpts(), opts...) {
		opt(cfg)
	}

	c := &consumer[T]{
		conn:    conn,
		log:     log,
		decoder: dec,
		handler: handler,
		config:  *cfg,
	}

	if cfg.jetStream {
		js, err := conn.JetStream()
		if err != nil {
			return nil, err
		}
		c.js = js
	}

	return c, nil
}

func (c *consumer[T]) Subscribe(ctx context.Context) error {
	// Store a derivable parent for the per-message context so Unsubscribe
	// can cancel in-flight handlers, and so the handler ctx carries any
	// values the caller injected (auth, request_id, etc.).
	baseCtx, cancel := context.WithCancel(ctx)
	c.baseCtxMu.Lock()
	c.baseCtx = baseCtx
	c.baseCancel = cancel
	c.baseCtxMu.Unlock()

	handler := c.dispatch
	var err error
	if c.js != nil {
		// JetStream
		if c.config.queueGroup != "" {
			c.sub, err = c.js.QueueSubscribe(c.config.subject, c.config.queueGroup, handler, c.config.jsOpts...)
		} else {
			c.sub, err = c.js.Subscribe(c.config.subject, handler, c.config.jsOpts...)
		}
	} else {
		// Core NATS
		if c.config.queueGroup != "" {
			c.sub, err = c.conn.Conn().QueueSubscribe(c.config.subject, c.config.queueGroup, handler)
		} else {
			c.sub, err = c.conn.Conn().Subscribe(c.config.subject, handler)
		}
	}

	if err != nil {
		cancel()
		return err
	}

	c.log.InfoContext(ctx, "nats:consumer -> subscribed", "subject", c.config.subject, "queue", c.config.queueGroup, "jetstream", c.config.jetStream)
	return nil
}

// dispatch decodes the message, invokes the user handler under a derived
// timeout, recovers from handler panics, and acks/nacks JetStream messages
// based on the result.
func (c *consumer[T]) dispatch(msg *nats.Msg) {
	c.baseCtxMu.RLock()
	parent := c.baseCtx
	c.baseCtxMu.RUnlock()
	if parent == nil {
		parent = context.Background()
	}
	if msg.Header == nil {
		msg.Header = nats.Header{}
	}
	parent = c.config.tracer.Extract(parent, headerCarrier(msg.Header))

	msgCtx, cancel := context.WithTimeout(parent, c.config.timeout)
	defer cancel()

	msgCtx, span := c.config.tracer.Start(msgCtx, "nats.consume "+msg.Subject,
		tracer.WithSpanKind(tracer.SpanKindConsumer),
		tracer.WithAttributes(
			tracer.String("messaging.system", "nats"),
			tracer.String("messaging.destination", msg.Subject),
		),
	)
	defer span.End()

	defer func() {
		if r := recover(); r != nil {
			err := recoveredErr(r)
			span.RecordError(err)
			span.SetStatus(tracer.StatusError, err.Error())
			c.log.ErrorContext(msgCtx, "nats:consumer -> handler panic", "err", err)
			c.reportErr(msgCtx, err)
			c.nakIfJS(msg)
		}
	}()

	event, err := c.decoder(msgCtx, msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(tracer.StatusError, err.Error())
		c.log.ErrorContext(msgCtx, "nats:consumer -> error decoding event", "err", err)
		c.reportErr(msgCtx, err)
		c.nakIfJS(msg)
		return
	}

	if err := c.handler.Handle(msgCtx, event); err != nil {
		span.RecordError(err)
		span.SetStatus(tracer.StatusError, err.Error())
		c.log.ErrorContext(msgCtx, "nats:consumer -> error handling event", "err", err)
		c.reportErr(msgCtx, err)
		c.nakIfJS(msg)
		return
	}

	c.ackIfJS(msg)
}

func (c *consumer[T]) reportErr(ctx context.Context, err error) {
	if c.config.onError != nil {
		c.config.onError(ctx, err)
	}
}

func (c *consumer[T]) ackIfJS(msg *nats.Msg) {
	if c.js == nil {
		return
	}
	if err := msg.Ack(); err != nil {
		c.log.Warn("nats:consumer -> ack failed", "err", err)
	}
}

func (c *consumer[T]) nakIfJS(msg *nats.Msg) {
	if c.js == nil {
		return
	}
	if err := msg.Nak(); err != nil {
		c.log.Warn("nats:consumer -> nak failed", "err", err)
	}
}

// recoveredErr normalises a recovered panic value into an error.
func recoveredErr(r any) error {
	if err, ok := r.(error); ok {
		return err
	}
	return fmt.Errorf("nats consumer panic: %v", r)
}

func (c *consumer[T]) Unsubscribe(ctx context.Context) error {
	c.baseCtxMu.Lock()
	cancel := c.baseCancel
	c.baseCancel = nil
	c.baseCtxMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if c.sub != nil {
		return c.sub.Unsubscribe()
	}
	return nil
}

type consumerConfig struct {
	subject    string
	queueGroup string
	jetStream  bool
	jsOpts     []nats.SubOpt
	timeout    time.Duration
	onError    OnError
	tracer     tracer.Tracer
}

type consumerOption func(*consumerConfig)

func WithQueueGroup(group string) consumerOption {
	return func(c *consumerConfig) {
		c.queueGroup = group
	}
}

func WithJetStream(opts ...nats.SubOpt) consumerOption {
	return func(c *consumerConfig) {
		c.jetStream = true
		c.jsOpts = opts
	}
}

// WithConsumerTimeout sets the per-message handler timeout.
func WithConsumerTimeout(d time.Duration) consumerOption {
	return func(c *consumerConfig) {
		c.timeout = d
	}
}

// WithOnError registers a callback invoked on decode + handle failures.
func WithOnError(fn OnError) consumerOption {
	return func(c *consumerConfig) {
		c.onError = fn
	}
}

// ConsumerWithTracer enables tracing on every received message — extracts
// the trace context from nats headers and wraps decoder + handler in a
// consumer span.
func ConsumerWithTracer(t tracer.Tracer) consumerOption {
	return func(c *consumerConfig) {
		if t != nil {
			c.tracer = t
		}
	}
}

// JSONDecoder is the JSON-payload decoder for NATS messages. It pulls
// bytes from the *nats.Msg envelope and hands them to transport.JSONDecoder
// so the JSON semantics stay in one place.
func JSONDecoder[T any](ctx context.Context, msg *nats.Msg) (T, error) {
	return transport.JSONDecoder[T](ctx, msg.Data)
}
