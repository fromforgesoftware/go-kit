// Package nats provides NATS / JetStream consumer + publisher helpers
// built on the broker-agnostic kit/transport interface.
package nats

import (
	"context"
	"fmt"

	"go.uber.org/fx"
)

// NewConnectionFx creates a new NATS connection module
func NewConnectionFx(opts ...connOption) fx.Option {
	return fx.Module(
		"nats:conn",
		fx.Provide(func() (Connection, error) {
			return NewConnection(opts...)
		}),
		fx.Invoke(func(lc fx.Lifecycle, conn Connection) {
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					conn.Close()
					return nil
				},
			})
		}),
	)
}

// NewProducerFx creates a new NATS producer module
func NewProducerFx[T any](constructor any, annotations ...fx.Annotation) fx.Option {
	return fx.Provide(
		fx.Annotate(
			constructor,
			append(
				annotations,
				fx.As(new(Producer[T])),
			)...,
		),
	)
}

// NewConsumerFx creates a new NATS consumer module
func NewConsumerFx[T any](constructor any, consumerName string, annotations ...fx.Annotation) fx.Option {
	return fx.Module(
		fmt.Sprintf("nats:consumer:%s", consumerName),
		fx.Provide(
			fx.Annotate(
				constructor,
				append(
					annotations,
					fx.As(new(Consumer)),
				)...,
			),
		),
		fx.Invoke(
			fx.Annotate(
				func(lc fx.Lifecycle, c Consumer) {
					lc.Append(fx.Hook{
						OnStart: func(ctx context.Context) error {
							return c.Subscribe(ctx)
						},
						OnStop: func(ctx context.Context) error {
							return c.Unsubscribe(ctx)
						},
					})
				},
			),
		),
	)
}

// FxModule provides the default NATS connection module.
func FxModule() fx.Option {
	return fx.Module("nats",
		NewConnectionFx(),
	)
}
