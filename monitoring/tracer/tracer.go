package tracer

import (
	"context"
	"strings"

	"github.com/fromforgesoftware/go-kit/monitoring/tracer/internal"
)

// TracerType defines the type of tracer implementation.
type TracerType string

const (
	OTelTracer TracerType = "otel"
	NoopTracer TracerType = "noop"
)

// ExporterType defines the kind of OTel exporter to install.
type ExporterType string

const (
	ExporterOTLPHTTP ExporterType = "otlphttp"
	ExporterOTLPGRPC ExporterType = "otlpgrpc"
	ExporterNone     ExporterType = "none"
)

// SpanKind mirrors OTel span kinds without forcing callers to import OTel.
type SpanKind int

const (
	SpanKindInternal SpanKind = iota
	SpanKindServer
	SpanKindClient
	SpanKindProducer
	SpanKindConsumer
)

// StatusCode mirrors OTel span status codes.
type StatusCode int

const (
	StatusUnset StatusCode = iota
	StatusOK
	StatusError
)

// Attribute is a typed key/value attached to spans and events.
type Attribute struct {
	Key   string
	Value any
}

// String returns an Attribute with a string value.
func String(key, value string) Attribute { return Attribute{Key: key, Value: value} }

// Int returns an Attribute with an int value.
func Int(key string, value int) Attribute { return Attribute{Key: key, Value: value} }

// Int64 returns an Attribute with an int64 value.
func Int64(key string, value int64) Attribute { return Attribute{Key: key, Value: value} }

// Float64 returns an Attribute with a float64 value.
func Float64(key string, value float64) Attribute { return Attribute{Key: key, Value: value} }

// Bool returns an Attribute with a bool value.
func Bool(key string, value bool) Attribute { return Attribute{Key: key, Value: value} }

// SpanContext identifies a span across processes.
type SpanContext struct {
	TraceID    string
	SpanID     string
	TraceFlags byte
	Remote     bool
}

// IsValid reports whether the SpanContext carries a usable trace id.
func (s SpanContext) IsValid() bool { return s.TraceID != "" && s.SpanID != "" }

// Carrier is the contract used by Inject/Extract to read or write trace
// headers from an arbitrary transport (HTTP headers, gRPC metadata, AMQP
// headers, NATS message headers, ...).
type Carrier interface {
	Get(key string) string
	Set(key, value string)
	Keys() []string
}

// MapCarrier adapts a string map to the Carrier interface. It is useful for
// tests and for transports that already expose headers as map[string]string.
type MapCarrier map[string]string

func (m MapCarrier) Get(key string) string { return m[key] }
func (m MapCarrier) Set(key, value string) { m[key] = value }
func (m MapCarrier) Keys() []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Span is the recording handle returned by Tracer.Start.
type Span interface {
	End()
	SetAttributes(attrs ...Attribute)
	SetStatus(code StatusCode, description string)
	RecordError(err error, attrs ...Attribute)
	AddEvent(name string, attrs ...Attribute)
	SpanContext() SpanContext
	IsRecording() bool
}

// StartOption configures a single Tracer.Start call.
type StartOption func(*StartConfig)

// StartConfig is the resolved set of options for Tracer.Start.
type StartConfig struct {
	Kind       SpanKind
	Attributes []Attribute
}

// WithSpanKind sets the kind of the started span.
func WithSpanKind(kind SpanKind) StartOption {
	return func(c *StartConfig) { c.Kind = kind }
}

// WithAttributes attaches initial attributes to the started span.
func WithAttributes(attrs ...Attribute) StartOption {
	return func(c *StartConfig) { c.Attributes = append(c.Attributes, attrs...) }
}

// Tracer is the surface every transport interacts with. Implementations must
// be safe for concurrent use.
type Tracer interface {
	// Start opens a new span as a child of any span already in ctx and returns
	// the derived context plus the span handle. Callers must End() the span.
	Start(ctx context.Context, name string, opts ...StartOption) (context.Context, Span)

	// SpanFromContext returns the span currently recorded in ctx, or a no-op
	// span if none is present.
	SpanFromContext(ctx context.Context) Span

	// Inject writes the current span context into carrier using the configured
	// propagator (defaults to W3C tracecontext + baggage).
	Inject(ctx context.Context, carrier Carrier)

	// Extract reads a span context from carrier and returns a context with it
	// installed as the remote parent.
	Extract(ctx context.Context, carrier Carrier) context.Context

	// Shutdown flushes any buffered spans and releases exporter resources.
	// Safe to call multiple times.
	Shutdown(ctx context.Context) error
}

// ParseType parses a string into a TracerType (case-insensitive). Unknown
// values fall through to OTelTracer so misconfiguration doesn't disable
// tracing silently in production.
func ParseType(s string) TracerType {
	switch strings.ToLower(s) {
	case "noop", "none", "disabled":
		return NoopTracer
	case "otel", "opentelemetry":
		return OTelTracer
	default:
		return OTelTracer
	}
}

// ParseExporter parses a string into an ExporterType. Unknown values fall
// through to ExporterOTLPHTTP.
func ParseExporter(s string) ExporterType {
	switch strings.ToLower(s) {
	case "otlphttp", "http", "":
		return ExporterOTLPHTTP
	case "otlpgrpc", "grpc":
		return ExporterOTLPGRPC
	case "none", "noop":
		return ExporterNone
	default:
		return ExporterOTLPHTTP
	}
}

// config is the resolved tracer configuration.
type config struct {
	tracerType   TracerType
	exporterType ExporterType
	serviceName  string
	endpoint     string
	insecure     bool
	sampleRatio  float64
	headers      map[string]string
	resourceAttr []Attribute
}

// Option configures the tracer at construction time.
type Option func(*config)

// WithType selects the tracer implementation (otel / noop).
func WithType(t TracerType) Option {
	return func(c *config) { c.tracerType = t }
}

// WithExporter selects the OTel exporter wire protocol.
func WithExporter(e ExporterType) Option {
	return func(c *config) { c.exporterType = e }
}

// WithServiceName sets the service.name resource attribute.
func WithServiceName(name string) Option {
	return func(c *config) { c.serviceName = name }
}

// WithEndpoint sets the OTLP exporter endpoint (host:port for grpc,
// scheme://host:port for http). Empty falls back to OTel SDK env vars.
func WithEndpoint(endpoint string) Option {
	return func(c *config) { c.endpoint = endpoint }
}

// WithInsecure disables transport security for the OTLP exporter.
func WithInsecure(insecure bool) Option {
	return func(c *config) { c.insecure = insecure }
}

// WithSampleRatio sets the head-based sampler ratio in [0,1]. 0 disables
// sampling, 1 samples everything.
func WithSampleRatio(ratio float64) Option {
	return func(c *config) { c.sampleRatio = ratio }
}

// WithHeaders attaches headers to the OTLP exporter (auth, tenant routing).
func WithHeaders(headers map[string]string) Option {
	return func(c *config) { c.headers = headers }
}

// WithResourceAttributes adds extra resource attributes (env, region, ...).
func WithResourceAttributes(attrs ...Attribute) Option {
	return func(c *config) { c.resourceAttr = append(c.resourceAttr, attrs...) }
}

func defaultConfig() []Option {
	return []Option{
		WithType(OTelTracer),
		WithExporter(ExporterOTLPHTTP),
		WithSampleRatio(1.0),
		WithInsecure(true),
	}
}

// New constructs a Tracer. Returns the tracer and a shutdown function. When
// type is NoopTracer the shutdown is a no-op.
func New(opts ...Option) (Tracer, error) {
	cfg := &config{}
	for _, opt := range append(defaultConfig(), opts...) {
		opt(cfg)
	}

	if cfg.tracerType == NoopTracer || cfg.exporterType == ExporterNone {
		return &tracerAdapter{Tracer: internal.NewNoopTracer()}, nil
	}

	t, err := internal.NewOTelTracer(internal.OTelConfig{
		ServiceName:        cfg.serviceName,
		Endpoint:           cfg.endpoint,
		Insecure:           cfg.insecure,
		SampleRatio:        cfg.sampleRatio,
		Headers:            cfg.headers,
		UseGRPC:            cfg.exporterType == ExporterOTLPGRPC,
		ResourceAttributes: toInternalAttrs(cfg.resourceAttr),
	})
	if err != nil {
		return nil, err
	}

	return &tracerAdapter{Tracer: t}, nil
}

func toInternalAttrs(in []Attribute) []internal.Attribute {
	out := make([]internal.Attribute, len(in))
	for i, a := range in {
		out[i] = internal.Attribute{Key: a.Key, Value: a.Value}
	}
	return out
}

func fromInternalCtx(in internal.SpanContext) SpanContext {
	return SpanContext{
		TraceID:    in.TraceID,
		SpanID:     in.SpanID,
		TraceFlags: in.TraceFlags,
		Remote:     in.Remote,
	}
}

// tracerAdapter bridges the internal tracer to the public Tracer interface,
// keeping OTel types out of the public surface.
type tracerAdapter struct {
	internal.Tracer
}

func (t *tracerAdapter) Start(ctx context.Context, name string, opts ...StartOption) (context.Context, Span) {
	cfg := &StartConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, sp := t.Tracer.Start(ctx, name, internal.StartConfig{
		Kind:       int(cfg.Kind),
		Attributes: toInternalAttrs(cfg.Attributes),
	})

	return ctx, &spanAdapter{Span: sp}
}

func (t *tracerAdapter) SpanFromContext(ctx context.Context) Span {
	return &spanAdapter{Span: t.Tracer.SpanFromContext(ctx)}
}

func (t *tracerAdapter) Inject(ctx context.Context, carrier Carrier) {
	t.Tracer.Inject(ctx, carrier)
}

func (t *tracerAdapter) Extract(ctx context.Context, carrier Carrier) context.Context {
	return t.Tracer.Extract(ctx, carrier)
}

// spanAdapter bridges the internal span to the public Span interface.
type spanAdapter struct {
	internal.Span
}

func (s *spanAdapter) SetAttributes(attrs ...Attribute) {
	s.Span.SetAttributes(toInternalAttrs(attrs)...)
}

func (s *spanAdapter) SetStatus(code StatusCode, description string) {
	s.Span.SetStatus(int(code), description)
}

func (s *spanAdapter) RecordError(err error, attrs ...Attribute) {
	s.Span.RecordError(err, toInternalAttrs(attrs)...)
}

func (s *spanAdapter) AddEvent(name string, attrs ...Attribute) {
	s.Span.AddEvent(name, toInternalAttrs(attrs)...)
}

func (s *spanAdapter) SpanContext() SpanContext {
	return fromInternalCtx(s.Span.SpanContext())
}
