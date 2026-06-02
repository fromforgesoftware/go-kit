package worker_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/fromforgesoftware/go-kit/app"
	"github.com/fromforgesoftware/go-kit/monitoring"
	"github.com/fromforgesoftware/go-kit/worker"
)

func TestRunWorker_BootsWithoutHTTP(t *testing.T) {
	t.Setenv("SVC_NAME", "worker-test")
	// Confirms RunWorker doesn't try to bind a REST address — no
	// REST_ADDRESS env var set.

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	err := app.RunContextWorker(ctx,
		app.WithName("smoke"),
		worker.FxModule(),
	)
	require.NoError(t, err)
}

func TestWorker_QueueOptionWiresHandler(t *testing.T) {
	t.Setenv("SVC_NAME", "worker-test")
	q := worker.NewInMemoryQueue()
	defer q.Close()

	var received atomic.Int32
	var wg sync.WaitGroup
	wg.Add(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		wg.Wait()
		// One message received — let the app shut down cleanly.
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// Publish from inside the fx graph once the queue is ready,
	// driven by an fx.Invoke that fires post-Start.
	err := app.RunContextWorker(ctx,
		app.WithName("smoke"),
		worker.FxModule(
			worker.WithBackend(q),
			worker.Queue[invoiceCmd]("invoices.create", func(_ context.Context, msg invoiceCmd) error {
				received.Add(1)
				if msg.CustomerID != "cust-1" {
					t.Errorf("unexpected customer: %q", msg.CustomerID)
				}
				wg.Done()
				return nil
			}),
		),
		fx.Invoke(func(lc fx.Lifecycle, pub worker.Publisher) {
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					// Sleep so the Subscribe goroutine inside the
					// worker module has time to register before we
					// publish (InMemoryQueue drops un-subscribed
					// publishes).
					go func() {
						time.Sleep(100 * time.Millisecond)
						_ = pub.Publish(context.Background(), "invoices.create",
							invoiceCmd{CustomerID: "cust-1", Amount: 100})
					}()
					return nil
				},
			})
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, int32(1), received.Load(), "queue handler should have fired exactly once")
}

func TestWorker_PublisherInjectableFromUsecase(t *testing.T) {
	t.Setenv("SVC_NAME", "worker-test")
	type deps struct {
		fx.In
		Pub worker.Publisher
	}
	var d deps
	a := fx.New(
		fx.NopLogger,
		monitoring.FxModule(),
		worker.FxModule(),
		fx.Populate(&d),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, a.Start(ctx))
	defer func() { _ = a.Stop(ctx) }()
	assert.NotNil(t, d.Pub, "worker.Publisher should be injectable")
}
