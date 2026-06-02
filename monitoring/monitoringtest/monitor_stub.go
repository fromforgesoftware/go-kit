package monitoringtest

import (
	"github.com/stretchr/testify/mock"

	"github.com/fromforgesoftware/go-kit/monitoring"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/monitoring/logger/loggertest"
	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
	"github.com/fromforgesoftware/go-kit/monitoring/tracer/tracertest"
)

// TestingT is satisfied by both *testing.T and *testing.B so benchmarks can
// reuse the stub monitor.
type TestingT interface {
	mock.TestingT
	Cleanup(func())
}

type monitorStubOpt func(m *monitor)

func WithLogger(l logger.Logger) monitorStubOpt {
	return func(m *monitor) {
		m.log = l
	}
}

func WithTracer(tr tracer.Tracer) monitorStubOpt {
	return func(m *monitor) {
		m.tr = tr
	}
}

type monitor struct {
	log logger.Logger
	tr  tracer.Tracer
}

func (m *monitor) Logger() logger.Logger {
	return m.log
}

func (m *monitor) Tracer() tracer.Tracer {
	return m.tr
}

func defaultMonitorOpts(t TestingT) []monitorStubOpt {
	return []monitorStubOpt{
		WithLogger(loggertest.NewStubLogger(t)),
		WithTracer(tracertest.NewStubTracer(t)),
	}
}

func NewMonitor(t TestingT, opts ...monitorStubOpt) monitoring.Monitor {
	m := new(monitor)
	for _, opt := range append(defaultMonitorOpts(t), opts...) {
		opt(m)
	}

	return m
}
