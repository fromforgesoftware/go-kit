package natstest

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/monitoring/logger/loggertest"
	"github.com/fromforgesoftware/go-kit/transport/nats"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StartEmbeddedServer starts an embedded NATS server for testing purposes.
// It returns the server instance. The server is automatically constrained to a random port.
// The caller is responsible for shutting down the server via t.Cleanup or defer.
func StartEmbeddedServer(t *testing.T) *server.Server {
	t.Helper()

	opts := &server.Options{
		Port: -1, // Random port
	}
	s, err := server.NewServer(opts)
	require.NoError(t, err)

	go s.Start()

	// Wait for server to be ready
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server failed to start")
	}

	t.Cleanup(func() {
		s.Shutdown()
	})

	return s
}

// NewConnectionForTest starts an embedded server and returns a client connection to it.
func NewConnectionForTest(t *testing.T) nats.Connection {
	t.Helper()

	s := StartEmbeddedServer(t)
	conn, err := nats.NewConnection(nats.WithConnURL(s.ClientURL()))
	require.NoError(t, err)

	t.Cleanup(func() {
		conn.Close()
	})

	return conn
}

type producerConstructor[P any] func(nats.Connection, logger.Logger) (nats.Producer[P], error)
type consumerConstructor[C any] func(nats.Connection, logger.Logger, nats.Handler[C]) (nats.Consumer, error)

// TestEncoderAndDecoder tests the nats passing the publisher and consumer.
//
// Note: The value T is needed because when a interface wraps another interface
// generics will not detect that interface C is composed by another interface.
func TestEncoderAndDecoder[P, C, T any](
	t *testing.T,
	producerCons producerConstructor[P],
	consumerCons consumerConstructor[C],
	obj P,
	assertObj func(*testing.T, T),
	opts ...nats.PublishOpt,
) {
	// NATS Transport uses nats.Connection interface wrapper
	conn := NewConnectionForTest(t)
	defer conn.Close()

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

	// Consumer Subscribe called manually usually? Or constructor creates inactive?
	// In NATS internal implementation (consumer.go), NewConsumer does NOT subscribe.
	// But `Invoke`/Fx lifecycle handles subscribe.
	// Here we must Subscribe manually.
	err = cons.Subscribe(context.Background())
	assert.NoError(t, err)

	err = prod.Publish(context.Background(), obj, opts...)
	assert.NoError(t, err)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}

	err = cons.Unsubscribe(context.Background())
	assert.NoError(t, err)
}

type testHandler[P, C any] struct {
	t         *testing.T
	assertObj func(*testing.T, C)
	wg        *sync.WaitGroup
}

func (h *testHandler[P, C]) Handle(ctx context.Context, receivedObj C) error {
	defer h.wg.Done()
	h.assertObj(h.t, receivedObj)
	return nil
}
