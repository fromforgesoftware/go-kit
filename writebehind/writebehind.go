package writebehind

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Flusher writes a batch of operations to the downstream store.
type Flusher[T any] interface {
	Flush(ctx context.Context, batch []T) error
}

// FlusherFunc adapts a function to Flusher.
type FlusherFunc[T any] func(ctx context.Context, batch []T) error

// Flush implements Flusher.
func (f FlusherFunc[T]) Flush(ctx context.Context, batch []T) error { return f(ctx, batch) }

// Policy controls behaviour when the queue is full.
type Policy uint8

const (
	// DropOldest replaces the oldest queued item with the new one.
	DropOldest Policy = iota
	// DropNewest drops the incoming item without queueing it.
	DropNewest
	// Block makes Push wait until queue space is available or ctx
	// expires.
	Block
	// CoalesceByKey overwrites an in-queue item that shares the same
	// key. Requires KeyFunc.
	CoalesceByKey
)

// Config configures a Queue.
type Config[T any] struct {
	Capacity      int                             // required; > 0
	BatchSize     int                             // max items per Flush; default 100
	FlushInterval time.Duration                   // periodic flush even when batch < BatchSize; default 1s
	Policy        Policy                          // default DropOldest
	KeyFunc       func(T) string                  // required when Policy == CoalesceByKey
	Flusher       Flusher[T]                      // required
	RetryBackoff  func(attempt int) time.Duration // default 100ms * 2^(attempt-1), capped 5s
	MaxRetries    int                             // default 3; -1 = infinite
	OnDrop        func(item T, reason string)
}

// Errors surfaced by the queue.
var (
	ErrCapacityRequired = errors.New("writebehind: Capacity must be > 0")
	ErrFlusherRequired  = errors.New("writebehind: Flusher is required")
	ErrCoalesceNeedsKey = errors.New("writebehind: Policy=CoalesceByKey requires KeyFunc")
	ErrQueueClosed      = errors.New("writebehind: queue closed")
	ErrPushBlockTimeout = errors.New("writebehind: Push timed out waiting for capacity")
)

// Queue is a typed write-behind queue. Goroutine-safe.
type Queue[T any] struct {
	cfg      Config[T]
	mu       sync.Mutex
	items    []T            // ordered slice; head at index 0
	keyIdx   map[string]int // for CoalesceByKey only
	notFull  *sync.Cond
	notEmpty *sync.Cond
	closed   atomic.Bool
	depth    atomic.Int64
}

// New constructs a Queue.
func New[T any](cfg Config[T]) (*Queue[T], error) {
	if cfg.Capacity <= 0 {
		return nil, ErrCapacityRequired
	}
	if cfg.Flusher == nil {
		return nil, ErrFlusherRequired
	}
	if cfg.Policy == CoalesceByKey && cfg.KeyFunc == nil {
		return nil, ErrCoalesceNeedsKey
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryBackoff == nil {
		cfg.RetryBackoff = defaultBackoff
	}
	q := &Queue[T]{cfg: cfg, items: make([]T, 0, cfg.Capacity), keyIdx: map[string]int{}}
	q.notFull = sync.NewCond(&q.mu)
	q.notEmpty = sync.NewCond(&q.mu)
	return q, nil
}

// Push enqueues item according to the configured Policy. Returns
// ErrQueueClosed after Run has returned, ErrPushBlockTimeout when
// Block-policy Push waits beyond ctx's deadline.
func (q *Queue[T]) Push(ctx context.Context, item T) error {
	if q.closed.Load() {
		return ErrQueueClosed
	}
	q.mu.Lock()

	if q.cfg.Policy == CoalesceByKey {
		key := q.cfg.KeyFunc(item)
		if idx, ok := q.keyIdx[key]; ok {
			q.items[idx] = item
			q.mu.Unlock()
			return nil
		}
	}

	for len(q.items) >= q.cfg.Capacity {
		switch q.cfg.Policy {
		case DropNewest:
			q.mu.Unlock()
			q.drop(item, "drop_newest")
			return nil
		case DropOldest:
			dropped := q.items[0]
			q.items = q.items[1:]
			q.depth.Add(-1)
			if q.cfg.OnDrop != nil {
				q.cfg.OnDrop(dropped, "drop_oldest")
			}
			// fall through to push
		case Block:
			waitDone := make(chan struct{})
			go func() {
				select {
				case <-ctx.Done():
					q.mu.Lock()
					q.notFull.Broadcast()
					q.mu.Unlock()
				case <-waitDone:
				}
			}()
			q.notFull.Wait()
			close(waitDone)
			if q.closed.Load() {
				q.mu.Unlock()
				return ErrQueueClosed
			}
			if err := ctx.Err(); err != nil {
				q.mu.Unlock()
				return ErrPushBlockTimeout
			}
			continue
		case CoalesceByKey:
			// CoalesceByKey did not find a slot — fall back to
			// DropOldest behaviour to bound memory.
			dropped := q.items[0]
			q.items = q.items[1:]
			q.depth.Add(-1)
			if q.cfg.OnDrop != nil {
				q.cfg.OnDrop(dropped, "drop_oldest")
			}
		}
	}

	q.items = append(q.items, item)
	q.depth.Add(1)
	if q.cfg.Policy == CoalesceByKey {
		q.keyIdx[q.cfg.KeyFunc(item)] = len(q.items) - 1
	}
	q.notEmpty.Signal()
	q.mu.Unlock()
	return nil
}

func (q *Queue[T]) drop(item T, reason string) {
	if q.cfg.OnDrop != nil {
		q.cfg.OnDrop(item, reason)
	}
}

// Depth returns the current queue length (atomic snapshot).
func (q *Queue[T]) Depth() int { return int(q.depth.Load()) }

// Run drains the queue until ctx is cancelled, flushing in batches
// of BatchSize or every FlushInterval (whichever fires first). On
// return, any remaining items are flushed one last time so shutdown
// is clean.
func (q *Queue[T]) Run(ctx context.Context) error {
	defer q.shutdown(ctx)

	t := time.NewTicker(q.cfg.FlushInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			q.drainOnce(ctx)
		default:
			batch := q.pull(ctx)
			if batch == nil {
				return ctx.Err()
			}
			if len(batch) == 0 {
				// Nothing pulled (closed); fall back to tick + ctx.
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-t.C:
					q.drainOnce(ctx)
				}
				continue
			}
			q.flushWithRetry(ctx, batch)
		}
	}
}

// pull blocks until at least one item is available, then drains up
// to BatchSize. Returns nil if ctx is done while waiting.
func (q *Queue[T]) pull(ctx context.Context) []T {
	q.mu.Lock()
	for len(q.items) == 0 {
		if q.closed.Load() {
			q.mu.Unlock()
			return nil
		}
		waitCh := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				q.mu.Lock()
				q.notEmpty.Broadcast()
				q.mu.Unlock()
			case <-waitCh:
			}
		}()
		q.notEmpty.Wait()
		close(waitCh)
		if ctx.Err() != nil {
			q.mu.Unlock()
			return nil
		}
	}
	n := q.cfg.BatchSize
	if n > len(q.items) {
		n = len(q.items)
	}
	batch := make([]T, n)
	copy(batch, q.items[:n])
	q.items = q.items[n:]
	q.depth.Add(-int64(n))
	if q.cfg.Policy == CoalesceByKey {
		// rebuild keyIdx — cheap because slice is bounded by Capacity.
		q.keyIdx = make(map[string]int, len(q.items))
		for i, it := range q.items {
			q.keyIdx[q.cfg.KeyFunc(it)] = i
		}
	}
	q.notFull.Broadcast()
	q.mu.Unlock()
	return batch
}

// drainOnce performs a single non-blocking flush of whatever's
// currently queued.
func (q *Queue[T]) drainOnce(ctx context.Context) {
	q.mu.Lock()
	if len(q.items) == 0 {
		q.mu.Unlock()
		return
	}
	n := q.cfg.BatchSize
	if n > len(q.items) {
		n = len(q.items)
	}
	batch := make([]T, n)
	copy(batch, q.items[:n])
	q.items = q.items[n:]
	q.depth.Add(-int64(n))
	if q.cfg.Policy == CoalesceByKey {
		q.keyIdx = make(map[string]int, len(q.items))
		for i, it := range q.items {
			q.keyIdx[q.cfg.KeyFunc(it)] = i
		}
	}
	q.notFull.Broadcast()
	q.mu.Unlock()
	q.flushWithRetry(ctx, batch)
}

func (q *Queue[T]) flushWithRetry(ctx context.Context, batch []T) {
	attempt := 0
	for {
		attempt++
		err := q.cfg.Flusher.Flush(ctx, batch)
		if err == nil {
			return
		}
		if q.cfg.MaxRetries >= 0 && attempt > q.cfg.MaxRetries {
			for _, item := range batch {
				q.drop(item, "retries_exhausted")
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(q.cfg.RetryBackoff(attempt)):
		}
	}
}

func (q *Queue[T]) shutdown(ctx context.Context) {
	q.closed.Store(true)
	q.mu.Lock()
	q.notEmpty.Broadcast()
	q.notFull.Broadcast()
	// Final drain — use a fresh background context so we still flush
	// even when ctx is already cancelled.
	remaining := q.items
	q.items = nil
	q.depth.Store(0)
	q.mu.Unlock()
	if len(remaining) == 0 {
		return
	}
	flushCtx := ctx
	if flushCtx.Err() != nil {
		flushCtx = context.Background()
	}
	q.flushWithRetry(flushCtx, remaining)
}

func defaultBackoff(attempt int) time.Duration {
	d := 100 * time.Millisecond
	for i := 1; i < attempt; i++ {
		d *= 2
		if d > 5*time.Second {
			return 5 * time.Second
		}
	}
	return d
}
