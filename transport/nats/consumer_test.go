package nats_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/fromforgesoftware/go-kit/monitoring/logger/loggertest"
	"github.com/fromforgesoftware/go-kit/transport/nats"
	natslib "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
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

func decodeTestObject(ctx context.Context, msg *natslib.Msg) (testObject, error) {
	var obj testObject
	err := json.Unmarshal(msg.Data, &obj)
	return obj, err
}

func helperNewConsumer(t *testing.T, conn nats.Connection, handler nats.Handler[testObject]) nats.Consumer {
	t.Helper()

	log := loggertest.NewStubLogger(t)
	cons, err := nats.NewConsumer(
		conn,
		log,
		"test.subject",
		decodeTestObject,
		handler,
	)
	assert.NoError(t, err)
	assert.NotNil(t, cons)
	return cons
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

	err := cons.Subscribe(context.Background())
	assert.NoError(t, err)

	wg.Add(1)
	err = prod.Publish(context.Background(), tObj)
	assert.NoError(t, err)

	wg.Add(1)
	err = prod.Publish(context.Background(), tObj)
	assert.NoError(t, err)

	wg.Wait()
	err = cons.Unsubscribe(context.Background())
	assert.NoError(t, err)
}
