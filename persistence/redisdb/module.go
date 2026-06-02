package redisdb

import (
	"context"

	"go.uber.org/fx"

	"github.com/fromforgesoftware/go-kit/monitoring"
)

func FxModule(options ...Option) fx.Option {
	return fx.Module(
		"redis",
		fx.Provide(func(m monitoring.Monitor) (*Client, error) {
			return New(m, options...)
		}),
		fx.Invoke(initLifecycle),
	)
}

func initLifecycle(lc fx.Lifecycle, cli *Client) error {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return cli.Close()
		},
	})

	return nil
}
