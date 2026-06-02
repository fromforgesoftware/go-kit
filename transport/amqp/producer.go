package amqp

import (
	"context"
	"maps"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
	"github.com/fromforgesoftware/go-kit/transport"
)

const defaultProducerTimeout = 5 * time.Second

type (
	MimeType string

	// Encoder is re-exported from kit/transport so a broker-agnostic
	// encoder function fits both AMQP and NATS publishers.
	Encoder[T any] = transport.Encoder[T]

	// encoder remains as a lower-case alias for the historical
	// unexported type; new code should use the exported Encoder.
	encoder[T any] = Encoder[T]

	Producer[T any] interface {
		Publish(ctx context.Context, v T, opts ...PublishOpt) error
	}

	publishConfig struct {
		overrideRoutingKey routingKey
	}

	PublishOpt func(*publishConfig)

	producer[T any] struct {
		*client
		encoder encoder[T]
		config  producerConfig
	}
)

const (
	MimeTypeJSON MimeType = "application/json"
)

func defaultPublishOpts() []ProducerOption {
	return []ProducerOption{
		ProducerWithMandatory(true),
		ProducerWithImmediate(false),
		ProducerWithHeaders(map[string]any{}),
		ProducerWithContentType(MimeTypeJSON),
		ProducerWithContentEncoding("utf-8"),
		ProducerWithDeliveryMode(amqp091.Persistent),
		ProducerWithPriority(0),
		ProducerWithAppID(""),
		ProducerWithTimeout(defaultProducerTimeout),
		ProducerWithTracer(noopTracer),
	}
}

func NewProducer[T any](
	conn Connection,
	log logger.Logger,
	exchange *Exchange,
	routingKey routingKey,
	enc encoder[T],
	opts ...ProducerOption,
) (Producer[T], error) {
	cfg := &producerConfig{routingKey: routingKey}
	for _, opt := range append(defaultPublishOpts(), opts...) {
		opt(cfg)
	}

	cli, err := newClient(conn, log, exchange)
	if err != nil {
		return nil, err
	}

	if err := cli.channel().Confirm(false); err != nil {
		return nil, err
	}

	return &producer[T]{
		client:  cli,
		encoder: enc,
		config:  *cfg,
	}, nil
}

func OverrideRoutingKey(parts ...RoutingKeyPart) PublishOpt {
	return func(p *publishConfig) {
		p.overrideRoutingKey = RoutingKey(parts...)
	}
}

func (p *producer[T]) Publish(ctx context.Context, v T, opts ...PublishOpt) error {
	cfg := &publishConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	body, err := p.encoder(ctx, v)
	if err != nil {
		return err
	}

	key := p.config.routingKey
	if cfg.overrideRoutingKey.String() != "" {
		key = cfg.overrideRoutingKey
	}

	publishCtx, cancel := context.WithTimeout(ctx, p.config.timeout)
	defer cancel()
	headers := maps.Clone(p.config.headers)
	if headers == nil {
		headers = amqp091.Table{}
	}

	publishCtx, span := p.config.tracer.Start(publishCtx, "amqp.publish "+p.exchange.name,
		tracer.WithSpanKind(tracer.SpanKindProducer),
		tracer.WithAttributes(
			tracer.String("messaging.system", "rabbitmq"),
			tracer.String("messaging.destination", p.exchange.name),
			tracer.String("messaging.rabbitmq.routing_key", key.String()),
		),
	)
	defer span.End()
	p.config.tracer.Inject(publishCtx, headerCarrier(headers))

	ch := p.channel()
	if ch == nil {
		span.SetStatus(tracer.StatusError, ErrConnectionClosed.Error())
		return ErrConnectionClosed
	}

	p.log.DebugContext(ctx,
		"producer about to publish message on exchange: %q, routingKey: %q, data: %q",
		p.exchange.name, key, string(body),
	)

	// amqp091.Channel.Publish is NOT safe for concurrent use — interleaved
	// frames break the wire protocol. Serialize all publishes on this
	// channel.
	p.publishMu.Lock()
	dConfirmation, err := ch.PublishWithDeferredConfirmWithContext(
		publishCtx,
		p.exchange.name,
		key.String(),
		p.config.mandatory,
		p.config.immediate,
		amqp091.Publishing{
			Headers:         headers,
			ContentType:     string(p.config.contentType),
			ContentEncoding: p.config.contentEncoding,
			DeliveryMode:    p.config.deliveryMode,
			Priority:        p.config.priority,
			AppId:           p.config.appID,
			Body:            body,
		},
	)
	p.publishMu.Unlock()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(tracer.StatusError, err.Error())
		return err
	}

	if _, err := dConfirmation.WaitContext(publishCtx); err != nil {
		span.RecordError(err)
		span.SetStatus(tracer.StatusError, err.Error())
		return err
	}
	p.log.DebugContext(ctx,
		"producer published message on exchange: %q, routingKey: %q, data: %q",
		p.exchange.name, key, string(body),
	)

	return nil
}

type producerConfig struct {
	mandatory       bool
	immediate       bool
	headers         map[string]any
	contentType     MimeType
	contentEncoding string
	deliveryMode    uint8
	priority        uint8
	appID           string
	timeout         time.Duration
	routingKey      routingKey
	tracer          tracer.Tracer
}

type ProducerOption func(*producerConfig)

func ProducerWithMandatory(mandatory bool) ProducerOption {
	return func(p *producerConfig) {
		p.mandatory = mandatory
	}
}

func ProducerWithImmediate(immediate bool) ProducerOption {
	return func(p *producerConfig) {
		p.immediate = immediate
	}
}

func ProducerWithHeaders(headers map[string]any) ProducerOption {
	return func(p *producerConfig) {
		p.headers = headers
	}
}

func ProducerWithContentType(contentType MimeType) ProducerOption {
	return func(p *producerConfig) {
		p.contentType = contentType
	}
}

func ProducerWithContentEncoding(contentEncoding string) ProducerOption {
	return func(p *producerConfig) {
		p.contentEncoding = contentEncoding
	}
}

func ProducerWithDeliveryMode(deliveryMode uint8) ProducerOption {
	return func(p *producerConfig) {
		p.deliveryMode = deliveryMode
	}
}

func ProducerWithPriority(priority uint8) ProducerOption {
	return func(p *producerConfig) {
		p.priority = priority
	}
}

func ProducerWithAppID(appID string) ProducerOption {
	return func(p *producerConfig) {
		p.appID = appID
	}
}

// ProducerWithTimeout sets the per-publish timeout.
func ProducerWithTimeout(timeout time.Duration) ProducerOption {
	return func(p *producerConfig) {
		p.timeout = timeout
	}
}

// PorducerWithTimeout is deprecated: use ProducerWithTimeout. Kept to
// avoid breaking existing callers; remove in a follow-up release.
func PorducerWithTimeout(timeout time.Duration) ProducerOption {
	return ProducerWithTimeout(timeout)
}

// ProducerWithTracer enables OpenTelemetry-style tracing on Publish. The
// tracer is used both to inject trace context into outgoing message headers
// and to wrap each publish in a producer span. When unset the package uses
// a no-op tracer so existing callers see no behaviour change.
func ProducerWithTracer(t tracer.Tracer) ProducerOption {
	return func(p *producerConfig) {
		if t != nil {
			p.tracer = t
		}
	}
}
