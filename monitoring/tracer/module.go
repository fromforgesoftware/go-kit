package tracer

import (
	"context"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/fx"
)

// Configuration is the env-driven config used by the Fx module. It honours
// the OTEL_* env vars where reasonable so it composes with the OpenTelemetry
// SDK conventions.
type Configuration struct {
	ServiceName string  `required:"true"  envconfig:"SVC_NAME"`
	Type        string  `required:"false" envconfig:"TRACER_TYPE"             default:"otel"`
	Exporter    string  `required:"false" envconfig:"TRACER_EXPORTER"         default:"otlphttp"`
	Endpoint    string  `required:"false" envconfig:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	Insecure    bool    `required:"false" envconfig:"TRACER_INSECURE"         default:"true"`
	SampleRatio float64 `required:"false" envconfig:"TRACER_SAMPLE_RATIO"     default:"1.0"`
	Enabled     bool    `required:"false" envconfig:"TRACER_ENABLED"          default:"true"`
}

// FxModule wires the tracer into an Fx application. The returned Tracer is
// shut down on fx.OnStop.
func FxModule() fx.Option {
	return fx.Module(
		"tracer",
		fx.Provide(fx.Annotate(newTracer, fx.As(new(Tracer)))),
	)
}

func newTracer(lc fx.Lifecycle) (Tracer, error) {
	cfg := Configuration{}
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}

	opts := []Option{
		WithServiceName(cfg.ServiceName),
		WithType(ParseType(cfg.Type)),
		WithExporter(ParseExporter(cfg.Exporter)),
		WithEndpoint(cfg.Endpoint),
		WithInsecure(cfg.Insecure),
		WithSampleRatio(cfg.SampleRatio),
	}
	if !cfg.Enabled {
		opts = append(opts, WithType(NoopTracer))
	}

	t, err := New(opts...)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return t.Shutdown(ctx)
		},
	})

	return t, nil
}
