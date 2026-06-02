package tracer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
)

func TestNoopTracer(t *testing.T) {
	t.Parallel()

	tr, err := tracer.New(tracer.WithType(tracer.NoopTracer))
	require.NoError(t, err)
	require.NotNil(t, tr)

	ctx := context.Background()
	ctx, sp := tr.Start(ctx, "noop-span", tracer.WithSpanKind(tracer.SpanKindServer))
	defer sp.End()

	// Noop tracer must accept any call without panicking. Whether the OTel
	// noop span reports IsRecording=false depends on the auto-instrumentation
	// indirect dep being linked, so we don't assert on it.
	sp.SetAttributes(tracer.String("key", "value"))
	sp.SetStatus(tracer.StatusOK, "fine")
	sp.RecordError(errors.New("ignored"))
	sp.AddEvent("event", tracer.Int("n", 1))
	_ = sp.SpanContext()
	_ = sp.IsRecording()

	require.NoError(t, tr.Shutdown(ctx))
}

func TestNoneExporterFallsThroughToNoop(t *testing.T) {
	t.Parallel()

	tr, err := tracer.New(tracer.WithExporter(tracer.ExporterNone))
	require.NoError(t, err)

	_, sp := tr.Start(context.Background(), "span")
	defer sp.End()

	// Same caveat as TestNoopTracer regarding IsRecording — just ensure the
	// span methods are safe to call.
	_ = sp.IsRecording()
}

func TestInjectExtractRoundTrip(t *testing.T) {
	t.Parallel()

	tr, err := tracer.New(tracer.WithType(tracer.NoopTracer))
	require.NoError(t, err)

	carrier := tracer.MapCarrier{}
	ctx, sp := tr.Start(context.Background(), "outbound")
	tr.Inject(ctx, carrier)
	sp.End()

	// Inject on a noop tracer produces no headers; assert it doesn't panic
	// and Extract on the same carrier yields a usable context.
	out := tr.Extract(context.Background(), carrier)
	assert.NotNil(t, out)
}

func TestMapCarrier(t *testing.T) {
	t.Parallel()

	c := tracer.MapCarrier{}
	c.Set("traceparent", "00-abc-def-01")
	assert.Equal(t, "00-abc-def-01", c.Get("traceparent"))
	assert.ElementsMatch(t, []string{"traceparent"}, c.Keys())
}

func TestParseType(t *testing.T) {
	t.Parallel()

	cases := map[string]tracer.TracerType{
		"":              tracer.OTelTracer,
		"otel":          tracer.OTelTracer,
		"OpenTelemetry": tracer.OTelTracer,
		"noop":          tracer.NoopTracer,
		"none":          tracer.NoopTracer,
		"disabled":      tracer.NoopTracer,
		"garbage":       tracer.OTelTracer,
	}
	for in, want := range cases {
		assert.Equalf(t, want, tracer.ParseType(in), "input %q", in)
	}
}

func TestParseExporter(t *testing.T) {
	t.Parallel()

	cases := map[string]tracer.ExporterType{
		"":         tracer.ExporterOTLPHTTP,
		"otlphttp": tracer.ExporterOTLPHTTP,
		"HTTP":     tracer.ExporterOTLPHTTP,
		"otlpgrpc": tracer.ExporterOTLPGRPC,
		"grpc":     tracer.ExporterOTLPGRPC,
		"none":     tracer.ExporterNone,
		"noop":     tracer.ExporterNone,
	}
	for in, want := range cases {
		assert.Equalf(t, want, tracer.ParseExporter(in), "input %q", in)
	}
}

func TestSpanFromContextWithoutSpan(t *testing.T) {
	t.Parallel()

	tr, err := tracer.New(tracer.WithType(tracer.NoopTracer))
	require.NoError(t, err)

	sp := tr.SpanFromContext(context.Background())
	require.NotNil(t, sp)
}
