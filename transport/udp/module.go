package udp

import (
	"context"

	"github.com/fromforgesoftware/go-kit/monitoring"
	"go.uber.org/fx"
)

// FxConfig holds optional configuration for the UDP module
type FxConfig struct {
	Controllers []Controller `ignored:"true"`
}

// FxModule provides the UDP server module — exports the Server and
// every Controller registered in the dependency graph.
func FxModule(opts ...serverOption) fx.Option {
	return fx.Module("transport_udp",
		fx.Provide(
			NewMux,
		),
		fx.Provide(
			fx.Annotate(
				WithControllers,
				fx.ParamTags(`group:"udpControllers"`),
				fx.ResultTags(`group:"udpGatewayOptions"`),
			),
		),
		fx.Provide(
			fx.Annotate(
				WithMiddlewares,
				fx.ParamTags(`group:"udpMiddlewares"`),
				fx.ResultTags(`group:"udpGatewayOptions"`),
			),
		),
		fx.Supply(
			fx.Annotate(
				opts,
				fx.ResultTags(`group:"udpGatewayOptions,flatten"`),
			),
		),
		fx.Invoke(
			fx.Annotate(func(lc fx.Lifecycle, monitor monitoring.Monitor, opts []serverOption) (*Server, error) {
				defaultOpts := []serverOption{
					WithAddressFromEnv("UDP_ADDRESS"),
				}

				allOpts := append(defaultOpts, opts...)

				s, err := NewServer(monitor, allOpts...)
				if err != nil {
					return nil, err
				}

				lc.Append(fx.Hook{
					OnStart: func(ctx context.Context) error {
						return s.Start()
					},
					OnStop: func(ctx context.Context) error {
						return s.Stop()
					},
				})

				return s, nil
			}, fx.ParamTags(``, ``, `group:"udpGatewayOptions"`)),
		),
	)
}

// NewFxController is a helper function to build a compatible and annotated fx module
func NewFxController(controller any) fx.Option {
	return fx.Provide(
		fx.Annotate(
			controller,
			fx.ResultTags(`group:"udpControllers"`),
			fx.As(new(Controller)),
		),
	)
}

// NewFxMiddleware is a helper function to build a compatible and annotated fx module
func NewFxMiddleware(middleware any) fx.Option {
	return fx.Provide(
		fx.Annotate(
			middleware,
			fx.ResultTags(`group:"udpMiddlewares"`),
			fx.As(new(Middleware)),
		),
	)
}
