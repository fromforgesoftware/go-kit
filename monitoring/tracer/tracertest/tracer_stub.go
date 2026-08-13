package tracertest

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
)

// TestingT is the interface wrapper around *testing.T and *testing.B.
type TestingT interface {
	mock.TestingT
	Cleanup(func())
}

// NewStubTracer returns a permissive mock Tracer that accepts any call and
// returns sensible zero values. Use it in unit tests where you don't care
// about tracing assertions.
func NewStubTracer(t TestingT) tracer.Tracer {
	tr := newUnassertedTracer(t)

	tr.EXPECT().Extract(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, _ tracer.Carrier) context.Context { return ctx }).Maybe()
	tr.EXPECT().Inject(mock.Anything, mock.Anything).Maybe()
	tr.EXPECT().SpanFromContext(mock.Anything).Return(NewStubSpan(t)).Maybe()
	tr.EXPECT().Shutdown(mock.Anything).Return(nil).Maybe()

	// Start accepts a variable number of StartOption args. Mockery generates a
	// variadic mock — register matchers for 0..5 options which covers every
	// call site in the kit today.
	startOpts := []any{}
	for i := 0; i < 6; i++ {
		tr.EXPECT().
			Start(mock.Anything, mock.Anything, startOpts...).
			RunAndReturn(func(ctx context.Context, _ string, _ ...tracer.StartOption) (context.Context, tracer.Span) {
				return ctx, NewStubSpan(t)
			}).
			Maybe()
		startOpts = append(startOpts, mock.Anything)
	}

	return tr
}

// NewStubSpan returns a permissive mock Span that accepts any call.
//
// It deliberately does not assert its expectations at cleanup. Every one of
// them is optional, so asserting adds nothing — but a stub span is built inside
// Start, which means one can be created by a goroutine that is still finishing
// as the test tears down. Registering an assertion then fails whichever test
// happens to own the stack, and the report names neither the goroutine nor the
// span. A permissive stub must not be able to fail a test.
func NewStubSpan(t TestingT) tracer.Span {
	sp := newUnassertedSpan(t)

	sp.EXPECT().End().Maybe()
	sp.EXPECT().IsRecording().Return(false).Maybe()
	sp.EXPECT().SpanContext().Return(tracer.SpanContext{}).Maybe()
	sp.EXPECT().SetStatus(mock.Anything, mock.Anything).Maybe()

	attrArgs := []any{}
	for i := 0; i < 10; i++ {
		sp.EXPECT().SetAttributes(attrArgs...).Maybe()
		sp.EXPECT().RecordError(mock.Anything, attrArgs...).Maybe()
		sp.EXPECT().AddEvent(mock.Anything, attrArgs...).Maybe()
		attrArgs = append(attrArgs, mock.Anything)
	}

	return sp
}

// newUnassertedSpan is NewSpan without the cleanup assertion. See NewStubSpan.
func newUnassertedSpan(t TestingT) *Span {
	m := &Span{}
	m.Mock.Test(t)
	return m
}

// newUnassertedTracer is NewTracer without the cleanup assertion: it hands out
// stub spans, so it carries the same hazard.
func newUnassertedTracer(t TestingT) *Tracer {
	m := &Tracer{}
	m.Mock.Test(t)
	return m
}
