// Package internal hides OTel SDK types behind a thin interface so the
// public tracer package can stay free of vendor-specific imports.
package internal

import "context"

// Attribute is the internal mirror of tracer.Attribute.
type Attribute struct {
	Key   string
	Value any
}

// SpanContext is the internal mirror of tracer.SpanContext.
type SpanContext struct {
	TraceID    string
	SpanID     string
	TraceFlags byte
	Remote     bool
}

// StartConfig is the internal mirror of tracer.StartConfig.
type StartConfig struct {
	Kind       int
	Attributes []Attribute
}

// Carrier is the internal mirror of tracer.Carrier.
type Carrier interface {
	Get(key string) string
	Set(key, value string)
	Keys() []string
}

// Span is the internal span surface.
type Span interface {
	End()
	SetAttributes(attrs ...Attribute)
	SetStatus(code int, description string)
	RecordError(err error, attrs ...Attribute)
	AddEvent(name string, attrs ...Attribute)
	SpanContext() SpanContext
	IsRecording() bool
}

// Tracer is the internal tracer surface.
type Tracer interface {
	Start(ctx context.Context, name string, cfg StartConfig) (context.Context, Span)
	SpanFromContext(ctx context.Context) Span
	Inject(ctx context.Context, carrier Carrier)
	Extract(ctx context.Context, carrier Carrier) context.Context
	Shutdown(ctx context.Context) error
}
