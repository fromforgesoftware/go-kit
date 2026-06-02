package worker_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/worker"
)

type invoiceCmd struct {
	CustomerID string `json:"customer_id"`
	Amount     int    `json:"amount"`
}

func TestInMemoryQueue_PublishDeliversToSubscriber(t *testing.T) {
	q := worker.NewInMemoryQueue()
	defer q.Close()

	var got []byte
	var wg sync.WaitGroup
	wg.Add(1)

	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = q.Subscribe(subCtx, "topic", func(_ context.Context, payload []byte) error {
			got = payload
			wg.Done()
			return nil
		})
	}()
	// Yield so the subscription is in place before publish.
	time.Sleep(10 * time.Millisecond)

	require.NoError(t, q.Publish(context.Background(), "topic", []byte(`hello`)))
	waitOrTimeout(t, &wg, time.Second)
	assert.Equal(t, "hello", string(got))
}

func TestPublisher_MarshalsTypedMessage(t *testing.T) {
	q := worker.NewInMemoryQueue()
	defer q.Close()

	var got string
	var wg sync.WaitGroup
	wg.Add(1)
	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = q.Subscribe(subCtx, "invoices.create", func(_ context.Context, payload []byte) error {
			got = string(payload)
			wg.Done()
			return nil
		})
	}()
	time.Sleep(10 * time.Millisecond)

	pub := worker.NewPublisher(q)
	require.NoError(t, pub.Publish(context.Background(), "invoices.create",
		invoiceCmd{CustomerID: "cust-1", Amount: 100}))
	waitOrTimeout(t, &wg, time.Second)
	assert.JSONEq(t, `{"customer_id":"cust-1","amount":100}`, got)
}

func waitOrTimeout(t *testing.T, wg *sync.WaitGroup, d time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("timed out after %s waiting for handler", d)
	}
}
