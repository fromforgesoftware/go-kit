package monitoring

import (
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
)

type Monitor interface {
	Logger() logger.Logger
	Tracer() tracer.Tracer
}

type monitor struct {
	l logger.Logger
	t tracer.Tracer
}

func (m *monitor) Logger() logger.Logger {
	return m.l
}

func (m *monitor) Tracer() tracer.Tracer {
	return m.t
}

func New(l logger.Logger, t tracer.Tracer) Monitor {
	if l == nil {
		panic("logger cannot be nil")
	}
	if t == nil {
		panic("tracer cannot be nil")
	}

	return &monitor{l: l, t: t}
}
