package middleware

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
)

// mdCarrier adapts grpc/metadata.MD to tracer.Carrier so the same tracer
// implementation can propagate trace context across HTTP, gRPC, AMQP and
// NATS without per-transport coupling in monitoring/.
type mdCarrier struct{ md metadata.MD }

func (c mdCarrier) Get(key string) string {
	v := c.md.Get(key)
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

func (c mdCarrier) Set(key, value string) {
	c.md.Set(key, value)
}

func (c mdCarrier) Keys() []string {
	out := make([]string, 0, len(c.md))
	for k := range c.md {
		out = append(out, k)
	}
	return out
}

// Tracing returns a server-side unary interceptor that extracts the trace
// context from incoming metadata, starts a server span for the duration of
// the handler, and records the resulting gRPC status code.
func Tracing(t tracer.Tracer) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		if md == nil {
			md = metadata.MD{}
		}
		ctx = t.Extract(ctx, mdCarrier{md: md})

		ctx, span := t.Start(ctx, info.FullMethod,
			tracer.WithSpanKind(tracer.SpanKindServer),
			tracer.WithAttributes(
				tracer.String("rpc.system", "grpc"),
				tracer.String("rpc.method", info.FullMethod),
			),
		)
		defer span.End()

		resp, err := handler(ctx, req)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(tracer.StatusError, err.Error())
		}
		return resp, err
	}
}

// ClientTracing returns a client-side unary interceptor that starts a span
// around the outbound call and injects the trace context into the outgoing
// metadata. Pair with grpc.Dial(..., grpc.WithChainUnaryInterceptor(...)).
func ClientTracing(t tracer.Tracer) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, span := t.Start(ctx, method,
			tracer.WithSpanKind(tracer.SpanKindClient),
			tracer.WithAttributes(
				tracer.String("rpc.system", "grpc"),
				tracer.String("rpc.method", method),
			),
		)
		defer span.End()

		md, _ := metadata.FromOutgoingContext(ctx)
		if md == nil {
			md = metadata.MD{}
		} else {
			md = md.Copy()
		}
		t.Inject(ctx, mdCarrier{md: md})
		ctx = metadata.NewOutgoingContext(ctx, md)

		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(tracer.StatusError, err.Error())
		}
		return err
	}
}
