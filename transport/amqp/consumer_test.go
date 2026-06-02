//go:build integration
// +build integration

package amqp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/go-kit/monitoring/logger/loggertest"
	"github.com/fromforgesoftware/go-kit/transport/amqp"
)

type validHandler struct {
	t   *testing.T
	obj testObject
	wg  *sync.WaitGroup
}

func (h *validHandler) Handle(ctx context.Context, receivedObj testObject) error {
	assert.Equal(h.t, h.obj, receivedObj)
	h.wg.Done()
	return nil
}

type sleepHandler struct {
	t     *testing.T
	sleep time.Duration
	wg    *sync.WaitGroup
}

func (h *sleepHandler) Handle(ctx context.Context, receivedObj testObject) error {
	time.Sleep(h.sleep)
	h.wg.Done()
	return nil
}

func decodeTestObject(ctx context.Context, b []byte) (testObject, error) {
	var obj testObject
	err := json.Unmarshal(b, &obj)
	return obj, err
}

func helperNewConsumer(t *testing.T, conn amqp.Connection, handler amqp.Handler[testObject]) amqp.Consumer {
	t.Helper()

	log := loggertest.NewStubLogger(t)
	cli, err := amqp.NewConsumer(
		conn,
		log,
		amqp.NewExchange("test-exchange", amqp.ExchangeTypeTopic),
		amqp.RoutingKey(amqp.RoutingKeyPart("test-queue"), amqp.RoutingKeyPartMatchAnyWords),
		amqp.NewQueue("consumerName", amqp.QueueName("test-queue")),
		decodeTestObject,
		handler,
	)
	assert.NoError(t, err)
	assert.NotNil(t, cli)
	return cli
}

func TestConsumerSubscribeWithProducer(t *testing.T) {
	conn := helperNewConnection(t)
	prod := helperNewProducer(t, conn)

	wg := sync.WaitGroup{}
	tObj := testObject{TestField: "test"}

	handler := &validHandler{
		t:   t,
		obj: tObj,
		wg:  &wg,
	}

	cons := helperNewConsumer(t, conn, handler)

	go cons.Subscribe(t.Context(), func(ctx context.Context, err error) {})

	wg.Add(1)
	err := prod.Publish(t.Context(), tObj)
	assert.NoError(t, err)

	wg.Add(1)
	err = prod.Publish(t.Context(), tObj)
	assert.NoError(t, err)

	wg.Wait()
	err = cons.Unsubscribe(t.Context())
	assert.NoError(t, err)
}

type errorHandler struct{}

func (h *errorHandler) Handle(ctx context.Context, receivedObj testObject) error {
	return assert.AnError
}

type panicHandler struct{}

func (h *panicHandler) Handle(ctx context.Context, receivedObj testObject) error {
	panic(fmt.Errorf("test panic in consumer handler"))
}

func TestConsumerSubscribeHandlerError(t *testing.T) {
	tests := []struct {
		name    string
		handler amqp.Handler[testObject]
		want    error
	}{
		{
			name:    "handler error",
			handler: &errorHandler{},
			want:    assert.AnError,
		},
		{
			name:    "panic error",
			handler: &panicHandler{},
			want:    fmt.Errorf("test panic in consumer handler"),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			conn := helperNewConnection(t)
			prod := helperNewProducer(t, conn)

			wg := sync.WaitGroup{}
			tObj := testObject{TestField: "test"}

			cons := helperNewConsumer(t, conn, test.handler)

			go cons.Subscribe(t.Context(), func(_ context.Context, err error) {
				wg.Done()
				assert.Equal(t, test.want, err)
			})

			wg.Add(1)
			err := prod.Publish(t.Context(), tObj)
			assert.NoError(t, err)

			wg.Wait()
			err = cons.Unsubscribe(t.Context())
			assert.NoError(t, err)
		})
	}
}

func TestConsumerConsumeTimeout(t *testing.T) {
	conn := helperNewConnection(t)
	prod := helperNewProducer(t, conn)

	wg := sync.WaitGroup{}
	tObj := testObject{TestField: "test"}
	handler := &sleepHandler{
		t:     t,
		sleep: 1 * time.Second,
		wg:    &wg,
	}

	// Inline creation to pass option
	log := loggertest.NewStubLogger(t)
	cons, err := amqp.NewConsumer(
		conn,
		log,
		amqp.NewExchange("test-exchange", amqp.ExchangeTypeTopic),
		amqp.RoutingKey(amqp.RoutingKeyPart("test-queue"), amqp.RoutingKeyPartMatchAnyWords),
		amqp.NewQueue("consumerName", amqp.QueueName("test-queue")),
		decodeTestObject,
		handler,
		amqp.WithTimeout(10*time.Millisecond),
	)
	assert.NoError(t, err)
	assert.NotNil(t, cons)

	go cons.Subscribe(t.Context(), func(_ context.Context, _ error) {})

	wg.Add(1)
	err = prod.Publish(t.Context(), tObj)
	assert.NoError(t, err)

	wg.Wait()
	err = cons.Unsubscribe(t.Context())
	assert.NoError(t, err)
}
