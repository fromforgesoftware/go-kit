package nats

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
	"github.com/fromforgesoftware/go-kit/transport"
)

const defaultProducerTimeout = 5 * time.Second

type (
	// Encoder is an alias for transport.Encoder so a broker-agnostic
	// encoder fits both NATS and AMQP producers.
	Encoder[T any] = transport.Encoder[T]

	Producer[T any] interface {
		Publish(ctx context.Context, v T, opts ...PublishOpt) error
	}

	publishConfig struct {
		subject string
	}

	PublishOpt func(*publishConfig)

	producer[T any] struct {
		conn    Connection
		encoder Encoder[T]
		log     logger.Logger
		config  producerConfig
		js      nats.JetStreamContext
	}
)

func defaultPublishOpts() []ProducerOption {
	return []ProducerOption{
		ProducerWithTimeout(defaultProducerTimeout),
		ProducerWithTracer(noopTracer),
	}
}

func NewProducer[T any](
	conn Connection,
	log logger.Logger,
	subject string,
	enc Encoder[T],
	opts ...ProducerOption,
) (Producer[T], error) {
	cfg := &producerConfig{subject: subject}
	for _, opt := range append(defaultPublishOpts(), opts...) {
		opt(cfg)
	}

	p := &producer[T]{
		conn:    conn,
		encoder: enc,
		log:     log,
		config:  *cfg,
	}

	if cfg.jetStream {
		js, err := conn.JetStream()
		if err != nil {
			return nil, err
		}
		p.js = js
	}

	return p, nil
}

func OverrideSubject(subject string) PublishOpt {
	return func(p *publishConfig) {
		p.subject = subject
	}
}

func (p *producer[T]) Publish(ctx context.Context, v T, opts ...PublishOpt) error {
	cfg := &publishConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	data, err := p.encoder(ctx, v)
	if err != nil {
		return err
	}

	subject := p.config.subject
	if cfg.subject != "" {
		subject = cfg.subject
	}

	p.log.DebugContext(ctx, "producer publishing message", "subject", subject)

	// Apply the configured timeout via ctx — previously the field existed
	// but was never wired, so JetStream publishes could hang forever
	// regardless of the timeout option.
	if p.config.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.config.timeout)
		defer cancel()
	}

	ctx, span := p.config.tracer.Start(ctx, "nats.publish "+subject,
		tracer.WithSpanKind(tracer.SpanKindProducer),
		tracer.WithAttributes(
			tracer.String("messaging.system", "nats"),
			tracer.String("messaging.destination", subject),
		),
	)
	defer span.End()

	msg := &nats.Msg{Subject: subject, Data: data, Header: nats.Header{}}
	p.config.tracer.Inject(ctx, headerCarrier(msg.Header))

	if p.js != nil {
		_, err = p.js.PublishMsg(msg, nats.Context(ctx))
	} else {
		// Core NATS PublishMsg is fire-and-forget but carries headers,
		// preserving trace context propagation downstream.
		err = p.conn.Conn().PublishMsg(msg)
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(tracer.StatusError, err.Error())
		return err
	}

	return nil
}

type producerConfig struct {
	subject   string
	timeout   time.Duration
	jetStream bool
	tracer    tracer.Tracer
}

type ProducerOption func(*producerConfig)

func ProducerWithTimeout(timeout time.Duration) ProducerOption {
	return func(p *producerConfig) {
		p.timeout = timeout
	}
}

func ProducerWithJetStream() ProducerOption {
	return func(p *producerConfig) {
		p.jetStream = true
	}
}

// ProducerWithTracer enables tracing on Publish — wraps the publish in a
// producer span and injects the trace context into nats.Msg headers.
func ProducerWithTracer(t tracer.Tracer) ProducerOption {
	return func(p *producerConfig) {
		if t != nil {
			p.tracer = t
		}
	}
}

// JSONEncoder is a thin alias for transport.JSONEncoder kept so existing
// imports continue to compile. New code can use transport.JSONEncoder
// directly.
func JSONEncoder[T any](ctx context.Context, v T) ([]byte, error) {
	return transport.JSONEncoder[T](ctx, v)
}
