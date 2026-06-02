package amqp

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
)

func NewConnectionFx(connOptions ...connOption) fx.Option {
	return fx.Module(
		"amqp:conn",
		fx.Provide(func(log logger.Logger) (Connection, error) {
			return NewConnection(log, connOptions...)
		}),
		fx.Invoke(connLifecycle),
	)
}

func connLifecycle(lc fx.Lifecycle, conn Connection) error {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return conn.Close()
		},
	})

	return nil
}

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

func NewConsumerWithoutInjectedHandlerFx[T any](constructor any, consumerParamTags []string, consumerName string, annotations ...fx.Annotation) fx.Option {
	return fx.Module(
		fmt.Sprintf("amqp:consumer:%s", consumerName),
		fx.Provide(
			fx.Annotate(
				constructor,
				append(
					annotations,
					fx.ParamTags(consumerParamTags...),
					fx.ResultTags(fmt.Sprintf("name:%q", consumerName), ``),
					fx.As(new(Consumer)),
				)...,
			),
		),
		fx.Invoke(
			fx.Annotate(
				consumerLifecycle,
				fx.ParamTags(``, fmt.Sprintf("name:%q", consumerName)),
			),
		),
	)
}

func NewConsumerFx[T any](constructor any, consumerName, handlerName string, annotations ...fx.Annotation) fx.Option {
	return fx.Module(
		fmt.Sprintf("amqp:consumer:%s", consumerName),
		fx.Provide(
			fx.Annotate(
				constructor,
				append(
					annotations,
					fx.ParamTags(``, ``, ``, fmt.Sprintf("name:%q", handlerName)),
					fx.ResultTags(fmt.Sprintf("name:%q", consumerName), ``),
					fx.As(new(Consumer)),
				)...,
			),
		),
		fx.Invoke(
			fx.Annotate(
				consumerLifecycle,
				fx.ParamTags(``, fmt.Sprintf("name:%q", consumerName)),
			),
		),
	)
}

func consumerLifecycle(lc fx.Lifecycle, consumer Consumer) error {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				// Errors are surfaced via the OnError callback, not the
				// return value (which a goroutine has no way to reach).
				_ = consumer.Subscribe(context.Background(), func(ctx context.Context, err error) {})
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return consumer.Unsubscribe(ctx)
		},
	})
	return nil
}
