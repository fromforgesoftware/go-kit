package ratelimit

import "go.uber.org/fx"

// FxModule provides a process-local Limiter. Services build their HTTP
// middleware / gRPC interceptor from it with a KeyFunc + Policy.
func FxModule() fx.Option {
	return fx.Module("ratelimit",
		fx.Provide(func() Limiter { return New(NewInMemoryStore()) }),
	)
}
