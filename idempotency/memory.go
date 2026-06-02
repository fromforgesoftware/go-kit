package idempotency

import (
	"context"
	"sync"
	"time"
)

// MemoryStore is an in-process Store suited for tests and
// single-instance services. Records are pruned on access (lazy
// expiry) — there is no background goroutine.
type MemoryStore struct {
	clock func() time.Time

	mu      sync.Mutex
	records map[string]*entry
}

type entry struct {
	cached   Cached
	expires  time.Time
	inFlight *inFlight
}

type inFlight struct {
	done chan struct{}
}

// MemoryConfig configures a MemoryStore.
type MemoryConfig struct {
	Clock func() time.Time
}

// NewMemoryStore constructs an empty MemoryStore.
func NewMemoryStore(cfg MemoryConfig) *MemoryStore {
	clk := cfg.Clock
	if clk == nil {
		clk = time.Now
	}
	return &MemoryStore{clock: clk, records: map[string]*entry{}}
}

// Reserve implements Store.
func (s *MemoryStore) Reserve(ctx context.Context, key, requestHash string, ttl time.Duration) (*Cached, bool, error) {
	s.mu.Lock()
	now := s.clock()
	e, ok := s.records[key]
	if ok && now.After(e.expires) && e.inFlight == nil {
		delete(s.records, key)
		ok = false
	}
	if ok {
		if e.inFlight != nil {
			// Concurrent request — wait for the in-flight to commit,
			// then return that result (subject to hash match).
			waitCh := e.inFlight.done
			s.mu.Unlock()
			select {
			case <-waitCh:
			case <-ctx.Done():
				return nil, false, ctx.Err()
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			e, ok = s.records[key]
			if !ok || e.inFlight != nil || e.cached.StoredAt.IsZero() {
				return nil, false, nil
			}
			cp := e.cached
			return &cp, false, nil
		}
		cp := e.cached
		s.mu.Unlock()
		return &cp, false, nil
	}
	s.records[key] = &entry{
		expires:  now.Add(ttl),
		inFlight: &inFlight{done: make(chan struct{})},
	}
	s.records[key].cached.RequestHash = requestHash
	s.mu.Unlock()
	return nil, true, nil
}

// Commit implements Store.
func (s *MemoryStore) Commit(_ context.Context, key, requestHash string, response Cached) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.records[key]
	if !ok {
		return nil
	}
	response.RequestHash = requestHash
	e.cached = response
	if e.inFlight != nil {
		close(e.inFlight.done)
		e.inFlight = nil
	}
	return nil
}

// Release implements Store.
func (s *MemoryStore) Release(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.records[key]
	if !ok {
		return nil
	}
	if e.inFlight != nil {
		close(e.inFlight.done)
	}
	delete(s.records, key)
	return nil
}
