package monitoring_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/go-kit/monitoring"
	"github.com/fromforgesoftware/go-kit/monitoring/logger/loggertest"
	"github.com/fromforgesoftware/go-kit/monitoring/tracer/tracertest"
)

func TestInvalidMonitor(t *testing.T) {
	t.Parallel()

	t.Run("no logger", func(t *testing.T) {
		assert.PanicsWithValue(
			t, "logger cannot be nil",
			func() {
				monitoring.New(nil, tracertest.NewStubTracer(t))
			},
		)
	})

	t.Run("no tracer", func(t *testing.T) {
		assert.PanicsWithValue(
			t, "tracer cannot be nil",
			func() {
				monitoring.New(loggertest.NewStubLogger(t), nil)
			},
		)
	})

	t.Run("valid logger and tracer", func(t *testing.T) {
		l := loggertest.NewStubLogger(t)
		tr := tracertest.NewStubTracer(t)
		m := monitoring.New(l, tr)
		assert.NotNil(t, m)
		assert.Equal(t, l, m.Logger())
		assert.Equal(t, tr, m.Tracer())
	})
}
