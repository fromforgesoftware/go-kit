package monitoring

import (
	"go.uber.org/fx"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
)

// FxModule wires logger, tracer, and the Monitor facade into an Fx
// application. Importing this module is the recommended way to consume
// observability primitives — callers who want a slimmer surface can compose
// logger.FxModule() and tracer.FxModule() directly.
func FxModule() fx.Option {
	return fx.Module(
		"monitoring",
		logger.FxModule(),
		tracer.FxModule(),
		fx.Provide(fx.Annotate(New, fx.As(new(Monitor)))),
	)
}
