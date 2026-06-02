package amqp

import (
	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
)

// headerCarrier adapts amqp091 message headers (which are amqp091.Table,
// i.e. map[string]any) to tracer.Carrier so propagators can be reused.
type headerCarrier amqp091.Table

func (h headerCarrier) Get(key string) string {
	v, ok := h[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func (h headerCarrier) Set(key, value string) {
	h[key] = value
}

func (h headerCarrier) Keys() []string {
	out := make([]string, 0, len(h))
	for k := range h {
		out = append(out, k)
	}
	return out
}

// noopTracer is the fallback used when no WithTracer option was supplied.
// Created once at package init to avoid per-publish/per-consume allocation.
var noopTracer = func() tracer.Tracer {
	t, _ := tracer.New(tracer.WithType(tracer.NoopTracer))
	return t
}()
