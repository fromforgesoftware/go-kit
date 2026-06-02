package nats_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/transport/nats"
	"github.com/fromforgesoftware/go-kit/transport/nats/natstest"
	natslib "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
)

func TestNatsPubSub(t *testing.T) {
	obj := "test message"

	natstest.TestEncoderAndDecoder(
		t,
		func(c nats.Connection, l logger.Logger) (nats.Producer[string], error) {
			return nats.NewProducer(c, l, "test.subject", func(ctx context.Context, v string) ([]byte, error) {
				return []byte(v), nil
			})
		},
		func(c nats.Connection, l logger.Logger, h nats.Handler[string]) (nats.Consumer, error) {
			return nats.NewConsumer(c, l, "test.subject", func(ctx context.Context, msg *natslib.Msg) (string, error) {
				return string(msg.Data), nil
			}, h)
		},
		obj,
		func(t *testing.T, received string) {
			assert.Equal(t, obj, received)
		},
	)
}
