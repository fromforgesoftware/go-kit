package nats

import (
	natsgo "github.com/nats-io/nats.go"

	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
)

// headerCarrier adapts nats.Header to tracer.Carrier so propagators can be
// reused across transports.
type headerCarrier natsgo.Header

func (h headerCarrier) Get(key string) string {
	return natsgo.Header(h).Get(key)
}

func (h headerCarrier) Set(key, value string) {
	natsgo.Header(h).Set(key, value)
}

func (h headerCarrier) Keys() []string {
	out := make([]string, 0, len(h))
	for k := range h {
		out = append(out, k)
	}
	return out
}

var noopTracer = func() tracer.Tracer {
	t, _ := tracer.New(tracer.WithType(tracer.NoopTracer))
	return t
}()
