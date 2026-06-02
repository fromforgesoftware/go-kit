package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// QueueBackend is the byte-level pub/sub the typed Queue[T] sugar
// wraps. Backends are responsible for delivery semantics (at-most-
// once / at-least-once), retry, and ack — the typed wrapper above
// only cares about codec + handler dispatch.
type QueueBackend interface {
	// Subscribe registers handler for topic. Implementations call
	// handler concurrently per message; handler errors should trigger
	// backend retry / DLQ per its own policy. Subscribe MUST return
	// when ctx is cancelled.
	Subscribe(ctx context.Context, topic string, handler func(context.Context, []byte) error) error

	// Publish ships payload to topic. Synchronous from the caller's
	// perspective — once Publish returns nil the message is durable
	// (for backends that promise that; in-mem is best-effort).
	Publish(ctx context.Context, topic string, payload []byte) error
}

// Publisher is the producer-side handle services inject into usecases
// that emit work. It's the same shape as QueueBackend.Publish but
// scoped to the publish capability so usecases don't see the
// subscribe surface.
type Publisher interface {
	Publish(ctx context.Context, topic string, msg any) error
}

// NewPublisher adapts a QueueBackend (typed-message-aware via JSON
// codec) for usecase injection.
func NewPublisher(b QueueBackend) Publisher { return &jsonPublisher{b: b} }

type jsonPublisher struct{ b QueueBackend }

func (p *jsonPublisher) Publish(ctx context.Context, topic string, msg any) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("worker.Publish: marshal: %w", err)
	}
	return p.b.Publish(ctx, topic, payload)
}

// InMemoryQueue is a process-local QueueBackend: useful for tests,
// single-instance services, and the worker package's own examples.
// Delivery is best-effort: if no subscriber is registered when a
// Publish happens, the message is dropped. Buffer is unbounded —
// fine for tests, never use in production.
type InMemoryQueue struct {
	mu      sync.Mutex
	subs    map[string][]func(context.Context, []byte) error
	closed  bool
	closeCh chan struct{}
}

// NewInMemoryQueue returns a fresh process-local queue.
func NewInMemoryQueue() *InMemoryQueue {
	return &InMemoryQueue{
		subs:    map[string][]func(context.Context, []byte) error{},
		closeCh: make(chan struct{}),
	}
}

func (q *InMemoryQueue) Subscribe(ctx context.Context, topic string, handler func(context.Context, []byte) error) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return fmt.Errorf("InMemoryQueue: closed")
	}
	q.subs[topic] = append(q.subs[topic], handler)
	q.mu.Unlock()

	// Block until ctx is done so the caller (worker registration
	// loop) sees a long-lived subscription.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-q.closeCh:
		return nil
	}
}

func (q *InMemoryQueue) Publish(ctx context.Context, topic string, payload []byte) error {
	q.mu.Lock()
	handlers := append([]func(context.Context, []byte) error(nil), q.subs[topic]...)
	q.mu.Unlock()
	for _, h := range handlers {
		// Fire-and-forget per subscriber; aggregate errors are not
		// surfaced (in-memory is best-effort by design).
		go func(h func(context.Context, []byte) error) { _ = h(ctx, payload) }(h)
	}
	return nil
}

// Close stops every active Subscribe loop.
func (q *InMemoryQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	close(q.closeCh)
}
