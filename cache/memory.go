package cache

import (
	"context"
	"sync"
	"time"
)

// Memory is an in-process Cache[V] with TTL eviction and single-flight
// loading. Suitable for low-cardinality / latency-sensitive caches that
// don't need cross-instance sharing. Thread-safe.
type Memory[V any] struct {
	mu       sync.Mutex
	entries  map[string]memoryEntry[V]
	inFlight map[string]*inflight[V]

	now func() time.Time
}

type memoryEntry[V any] struct {
	value     V
	expiresAt time.Time // zero = no expiry
}

type inflight[V any] struct {
	wg    sync.WaitGroup
	value V
	err   error
}

func NewMemory[V any]() *Memory[V] {
	return &Memory[V]{
		entries:  make(map[string]memoryEntry[V]),
		inFlight: make(map[string]*inflight[V]),
		now:      time.Now,
	}
}

func (m *Memory[V]) Get(ctx context.Context, key string) (V, error) {
	if err := ctx.Err(); err != nil {
		var zero V
		return zero, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[key]
	if !ok || m.expired(e) {
		if ok {
			delete(m.entries, key)
		}
		var zero V
		return zero, ErrMiss
	}
	return e.value, nil
}

func (m *Memory[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	expiresAt := time.Time{}
	if ttl > 0 {
		expiresAt = m.now().Add(ttl)
	}
	m.entries[key] = memoryEntry[V]{value: value, expiresAt: expiresAt}
	return nil
}

func (m *Memory[V]) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key)
	return nil
}

func (m *Memory[V]) Clear(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make(map[string]memoryEntry[V])
	return nil
}

func (m *Memory[V]) GetOrLoad(
	ctx context.Context,
	key string,
	ttl time.Duration,
	loader func(ctx context.Context) (V, error),
) (V, error) {
	if v, err := m.Get(ctx, key); err == nil {
		return v, nil
	}

	m.mu.Lock()
	if pending, ok := m.inFlight[key]; ok {
		m.mu.Unlock()
		pending.wg.Wait()
		return pending.value, pending.err
	}
	pending := &inflight[V]{}
	pending.wg.Add(1)
	m.inFlight[key] = pending
	m.mu.Unlock()

	value, err := loader(ctx)
	m.mu.Lock()
	delete(m.inFlight, key)
	if err == nil {
		expiresAt := time.Time{}
		if ttl > 0 {
			expiresAt = m.now().Add(ttl)
		}
		m.entries[key] = memoryEntry[V]{value: value, expiresAt: expiresAt}
	}
	pending.value = value
	pending.err = err
	m.mu.Unlock()
	pending.wg.Done()
	return value, err
}

// Len returns the current entry count (live + expired). Useful for tests.
func (m *Memory[V]) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

// SetClock replaces the time source. Use in tests to drive expiry without
// real sleeps.
func (m *Memory[V]) SetClock(now func() time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = now
}

func (m *Memory[V]) expired(e memoryEntry[V]) bool {
	return !e.expiresAt.IsZero() && !m.now().Before(e.expiresAt)
}
