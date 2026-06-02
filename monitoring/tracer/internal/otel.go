package internal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	otelnoop "go.opentelemetry.io/otel/trace/noop"
)

// OTelConfig is the resolved configuration for the OTel impl.
type OTelConfig struct {
	ServiceName        string
	Endpoint           string
	Insecure           bool
	SampleRatio        float64
	Headers            map[string]string
	UseGRPC            bool
	ResourceAttributes []Attribute
}

// NewOTelTracer builds an OTel-backed Tracer with an OTLP exporter and the
// W3C tracecontext + baggage propagator.
func NewOTelTracer(cfg OTelConfig) (Tracer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var (
		exporter sdktrace.SpanExporter
		err      error
	)
	if cfg.UseGRPC {
		exporter, err = newOTLPGRPCExporter(ctx, cfg)
	} else {
		exporter, err = newOTLPHTTPExporter(ctx, cfg)
	}
	if err != nil {
		return nil, fmt.Errorf("create otlp exporter: %w", err)
	}

	res, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("build otel resource: %w", err)
	}

	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)

	// Install globally so third-party libs (otelhttp, otelgrpc) pick it up.
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(prop)

	return &otelTracer{
		tp:         tp,
		tracer:     tp.Tracer(cfg.ServiceName),
		propagator: prop,
	}, nil
}

// NewNoopTracer returns a Tracer that does nothing — useful when tracing is
// explicitly disabled.
func NewNoopTracer() Tracer {
	return &otelTracer{
		tp:         nil,
		tracer:     otelnoop.NewTracerProvider().Tracer(""),
		propagator: propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
	}
}

func newOTLPHTTPExporter(ctx context.Context, cfg OTelConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracehttp.Option{}
	if cfg.Endpoint != "" {
		opts = append(opts, otlptracehttp.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
	}
	return otlptrace.New(ctx, otlptracehttp.NewClient(opts...))
}

func newOTLPGRPCExporter(ctx context.Context, cfg OTelConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{}
	if cfg.Endpoint != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
	}
	return otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
}

func buildResource(ctx context.Context, cfg OTelConfig) (*sdkresource.Resource, error) {
	attrs := make([]attribute.KeyValue, 0, len(cfg.ResourceAttributes)+1)
	if cfg.ServiceName != "" {
		attrs = append(attrs, semconv.ServiceName(cfg.ServiceName))
	}
	for _, a := range cfg.ResourceAttributes {
		attrs = append(attrs, toOTelAttr(a))
	}

	base, err := sdkresource.New(ctx,
		sdkresource.WithFromEnv(),
		sdkresource.WithProcess(),
		sdkresource.WithTelemetrySDK(),
		sdkresource.WithAttributes(attrs...),
	)
	// sdkresource.New returns a partial resource alongside a schema-merge error
	// for some env combinations; tolerate that case so the SDK still works.
	if err != nil && !errors.Is(err, sdkresource.ErrPartialResource) {
		return nil, err
	}
	return base, nil
}

type otelTracer struct {
	tp         *sdktrace.TracerProvider
	tracer     oteltrace.Tracer
	propagator propagation.TextMapPropagator
}

func (t *otelTracer) Start(ctx context.Context, name string, cfg StartConfig) (context.Context, Span) {
	startOpts := []oteltrace.SpanStartOption{
		oteltrace.WithSpanKind(toOTelKind(cfg.Kind)),
	}
	if len(cfg.Attributes) > 0 {
		startOpts = append(startOpts, oteltrace.WithAttributes(toOTelAttrs(cfg.Attributes)...))
	}

	ctx, sp := t.tracer.Start(ctx, name, startOpts...)
	return ctx, &otelSpan{sp: sp}
}

func (t *otelTracer) SpanFromContext(ctx context.Context) Span {
	return &otelSpan{sp: oteltrace.SpanFromContext(ctx)}
}

func (t *otelTracer) Inject(ctx context.Context, carrier Carrier) {
	t.propagator.Inject(ctx, carrierAdapter{c: carrier})
}

func (t *otelTracer) Extract(ctx context.Context, carrier Carrier) context.Context {
	return t.propagator.Extract(ctx, carrierAdapter{c: carrier})
}

func (t *otelTracer) Shutdown(ctx context.Context) error {
	if t.tp == nil {
		return nil
	}
	return t.tp.Shutdown(ctx)
}

type otelSpan struct{ sp oteltrace.Span }

func (s *otelSpan) End() { s.sp.End() }

func (s *otelSpan) SetAttributes(attrs ...Attribute) {
	s.sp.SetAttributes(toOTelAttrs(attrs)...)
}

func (s *otelSpan) SetStatus(code int, description string) {
	s.sp.SetStatus(toOTelStatus(code), description)
}

func (s *otelSpan) RecordError(err error, attrs ...Attribute) {
	if err == nil {
		return
	}
	if len(attrs) > 0 {
		s.sp.RecordError(err, oteltrace.WithAttributes(toOTelAttrs(attrs)...))
		return
	}
	s.sp.RecordError(err)
}

func (s *otelSpan) AddEvent(name string, attrs ...Attribute) {
	if len(attrs) > 0 {
		s.sp.AddEvent(name, oteltrace.WithAttributes(toOTelAttrs(attrs)...))
		return
	}
	s.sp.AddEvent(name)
}

func (s *otelSpan) SpanContext() SpanContext {
	sc := s.sp.SpanContext()
	return SpanContext{
		TraceID:    sc.TraceID().String(),
		SpanID:     sc.SpanID().String(),
		TraceFlags: byte(sc.TraceFlags()),
		Remote:     sc.IsRemote(),
	}
}

func (s *otelSpan) IsRecording() bool { return s.sp.IsRecording() }

// carrierAdapter bridges the package Carrier to the OTel TextMapCarrier interface.
type carrierAdapter struct{ c Carrier }

func (a carrierAdapter) Get(key string) string { return a.c.Get(key) }
func (a carrierAdapter) Set(key, value string) { a.c.Set(key, value) }
func (a carrierAdapter) Keys() []string        { return a.c.Keys() }

func toOTelKind(k int) oteltrace.SpanKind {
	switch k {
	case 1:
		return oteltrace.SpanKindServer
	case 2:
		return oteltrace.SpanKindClient
	case 3:
		return oteltrace.SpanKindProducer
	case 4:
		return oteltrace.SpanKindConsumer
	default:
		return oteltrace.SpanKindInternal
	}
}

func toOTelStatus(c int) codes.Code {
	switch c {
	case 1:
		return codes.Ok
	case 2:
		return codes.Error
	default:
		return codes.Unset
	}
}

func toOTelAttrs(in []Attribute) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(in))
	for _, a := range in {
		out = append(out, toOTelAttr(a))
	}
	return out
}

func toOTelAttr(a Attribute) attribute.KeyValue {
	switch v := a.Value.(type) {
	case string:
		return attribute.String(a.Key, v)
	case bool:
		return attribute.Bool(a.Key, v)
	case int:
		return attribute.Int(a.Key, v)
	case int64:
		return attribute.Int64(a.Key, v)
	case float64:
		return attribute.Float64(a.Key, v)
	default:
		return attribute.String(a.Key, fmt.Sprintf("%v", v))
	}
}
