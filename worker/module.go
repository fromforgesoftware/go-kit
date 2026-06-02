package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/fx"

	"github.com/fromforgesoftware/go-kit/cron"
	"github.com/fromforgesoftware/go-kit/monitoring/logger"
	"github.com/fromforgesoftware/go-kit/monitoring/tracer"
)

// Option configures the worker FxModule.
type Option func(*config)

type config struct {
	backend QueueBackend
	cronJob []cronJob
	queue   []queueSub
}

type cronJob struct {
	interval time.Duration
	name     string
	fn       func(context.Context) error
}

type queueSub struct {
	topic   string
	wrapped func(context.Context, []byte) error
}

// WithBackend selects the queue backend. Defaults to NewInMemoryQueue().
func WithBackend(b QueueBackend) Option {
	return func(c *config) { c.backend = b }
}

// Cron registers an interval-based job. Wraps kit/cron.
//
//	worker.Cron(2*time.Hour, "daily-rollup", dailyRollup)
func Cron(interval time.Duration, name string, fn func(context.Context) error) Option {
	return func(c *config) {
		c.cronJob = append(c.cronJob, cronJob{interval: interval, name: name, fn: fn})
	}
}

// Queue subscribes handler to topic. Messages on the wire are JSON;
// the handler receives them decoded as T. Marshal/unmarshal errors
// are logged but don't crash the consumer.
//
//	type CreateInvoiceCmd struct { CustomerID string `json:"customer_id"` }
//	worker.Queue[CreateInvoiceCmd]("invoices.create", createInvoice)
func Queue[T any](topic string, handler func(context.Context, T) error) Option {
	wrapped := func(ctx context.Context, payload []byte) error {
		var msg T
		if err := json.Unmarshal(payload, &msg); err != nil {
			return fmt.Errorf("worker.Queue[%T] decode: %w", msg, err)
		}
		return handler(ctx, msg)
	}
	return func(c *config) {
		c.queue = append(c.queue, queueSub{topic: topic, wrapped: wrapped})
	}
}

// FxModule wires the worker runtime into the fx graph. Cron jobs
// start on app.Start; Queue subscriptions start their loops and
// drop on app.Stop.
//
// The module provides:
//
//   - worker.QueueBackend  (the configured backend, default InMemoryQueue)
//   - worker.Publisher     (typed-message-aware producer handle)
func FxModule(opts ...Option) fx.Option {
	c := &config{}
	for _, o := range opts {
		o(c)
	}
	if c.backend == nil {
		c.backend = NewInMemoryQueue()
	}

	return fx.Module("worker",
		// Provide the backend as its interface type so consumers can
		// inject either QueueBackend or Publisher without caring
		// about the concrete implementation.
		fx.Provide(func() QueueBackend { return c.backend }),
		fx.Provide(func(b QueueBackend) Publisher { return NewPublisher(b) }),
		fx.Invoke(func(lc fx.Lifecycle, log logger.Logger, _ tracer.Tracer) {
			// Cron jobs run via kit/cron.Scheduler — one scheduler
			// per worker module is enough; the kit handles per-job
			// goroutines + panic recovery.
			var sched *cron.Scheduler
			if len(c.cronJob) > 0 {
				sched = cron.New(cron.WithLogger(func(format string, args ...any) {
					log.Info(fmt.Sprintf(format, args...))
				}))
				for _, j := range c.cronJob {
					sched.Every(j.interval, j.name, j.fn)
				}
			}

			// Queue subscriptions each spawn their own goroutine that
			// blocks inside backend.Subscribe until ctx is cancelled.
			subCtx, cancelSubs := context.WithCancel(context.Background())
			var subsWG sync.WaitGroup

			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					if sched != nil {
						sched.Start(subCtx)
					}
					for _, sub := range c.queue {
						subsWG.Add(1)
						go func(s queueSub) {
							defer subsWG.Done()
							if err := c.backend.Subscribe(subCtx, s.topic, s.wrapped); err != nil && !errIsContextCancelled(err) {
								log.Error("worker.Queue subscribe ended",
									"topic", s.topic, "err", err)
							}
						}(sub)
					}
					return nil
				},
				OnStop: func(stopCtx context.Context) error {
					cancelSubs()
					if sched != nil {
						sched.Stop()
					}
					done := make(chan struct{})
					go func() { subsWG.Wait(); close(done) }()
					select {
					case <-done:
						return nil
					case <-stopCtx.Done():
						return stopCtx.Err()
					}
				},
			})
		}),
	)
}

func errIsContextCancelled(err error) bool {
	return err != nil && (err == context.Canceled || err.Error() == context.Canceled.Error())
}
