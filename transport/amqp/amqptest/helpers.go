//go:build integration
// +build integration

package amqptest

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/monitoring/logger/loggertest"
	"github.com/fromforgesoftware/go-kit/transport/amqp"
)

type producerConstructor[P any] func(amqp.Connection, logger.Logger) (amqp.Producer[P], error)

func getAMQPConnection(t *testing.T) amqp.Connection {
	t.Helper()

	url := GetRabbitMQURL(t)
	t.Setenv("AMQP_URL", url)
	conn, err := amqp.NewConnection(loggertest.NewStubLogger(t))
	require.NoError(t, err)

	return conn
}

// TestEncoderAndDecoder tests the amqp passing the publisher and consumer.
//
// Note: The value T is needed because when a interface wraps another interface
// generics will not detect that interface C is composed by another interface.
func TestEncoderAndDecoder[P, C, T any](
	t *testing.T,
	producerCons producerConstructor[P],
	consumerCons func(amqp.Connection, logger.Logger, amqp.Handler[C]) (amqp.Consumer, error),
	obj P,
	assertObj func(*testing.T, T),
	opts ...amqp.PublishOpt,
) {
	conn := getAMQPConnection(t)

	log := loggertest.NewStubLogger(t)
	prod, err := producerCons(conn, log)
	assert.NoError(t, err)

	wg := sync.WaitGroup{}
	handler := &testHandler[P, C]{
		t: t,
		assertObj: func(t *testing.T, c C) { // Needed to use cast types
			t.Helper()

			defer func() {
				if r := recover(); r != nil {
					t.Errorf("recovered from panic: %v", r)
				}
			}()
			var x any = c
			assertObj(t, x.(T))
		},
		wg: &wg,
	}
	cons, err := consumerCons(conn, log, handler)
	assert.NoError(t, err)
	wg.Add(1)

	err = prod.Publish(t.Context(), obj, opts...)
	assert.NoError(t, err)

	go cons.Subscribe(t.Context(), func(ctx context.Context, err error) {})

	wg.Wait()
	err = cons.Unsubscribe(t.Context())
	assert.NoError(t, err)
}

func NewProducerForTest[P any](t *testing.T, cons producerConstructor[P]) amqp.Producer[P] {
	t.Helper()

	conn := getAMQPConnection(t)

	log := loggertest.NewStubLogger(t)
	prod, err := cons(conn, log)
	require.NoError(t, err)

	return prod
}

type testHandler[P, C any] struct {
	t         *testing.T
	assertObj func(*testing.T, C)
	wg        *sync.WaitGroup
}

func (h *testHandler[P, C]) Handle(ctx context.Context, receivedObj C) error {
	h.assertObj(h.t, receivedObj)
	h.wg.Done()
	return nil
}
