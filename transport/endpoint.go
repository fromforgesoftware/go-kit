// Package transport defines the cross-cutting Endpoint + Error
// abstractions every concrete transport (grpc, rest, amqp, nats, tcp,
// udp, websocket) implements.
package transport

import "context"

type (
	Endpoint[I, O any]      func(ctx context.Context, request I) (O, error)
	AnyEndpoint             func(ctx context.Context, request interface{}) (interface{}, error)
	EmptyResEndpoint[I any] func(ctx context.Context, request I) error
)
