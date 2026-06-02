package ratelimit

import (
	"context"
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	last   time.Time
}

// inMemoryStore is a process-local token-bucket store. Suitable for
// bounded key spaces (per-provider throttles, single-replica services).
// For large key spaces or multi-replica fleets, use a Redis store.
type inMemoryStore struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

// NewInMemoryStore returns a process-local token-bucket Store.
func NewInMemoryStore() Store {
	return &inMemoryStore{buckets: make(map[string]*bucket)}
}

func (s *inMemoryStore) Take(_ context.Context, key string, p Policy, cost int, now time.Time) (Result, error) {
	if cost < 1 {
		cost = 1
	}
	capacity := p.Capacity()
	rate := p.RefillPerSecond()

	s.mu.Lock()
	defer s.mu.Unlock()

	b := s.buckets[key]
	if b == nil {
		b = &bucket{tokens: capacity, last: now}
		s.buckets[key] = b
	} else if elapsed := now.Sub(b.last).Seconds(); elapsed > 0 {
		b.tokens = min(capacity, b.tokens+elapsed*rate)
		b.last = now
	}

	res := Result{Limit: p.Limit}
	want := float64(cost)
	if b.tokens >= want {
		b.tokens -= want
		res.Allowed = true
	} else if rate > 0 {
		res.RetryAfter = secondsToDuration((want - b.tokens) / rate)
	} else {
		res.RetryAfter = -1 // unreachable: no refill configured
	}
	res.Remaining = int(b.tokens)
	if rate > 0 {
		res.ResetAfter = secondsToDuration((capacity - b.tokens) / rate)
	}
	return res, nil
}

// Purge drops buckets idle (no take) for at least idleFor, bounding memory in
// large key spaces. Safe to call periodically from a worker/cron.
func (s *inMemoryStore) Purge(idleFor time.Duration, now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for k, b := range s.buckets {
		if now.Sub(b.last) >= idleFor {
			delete(s.buckets, k)
			n++
		}
	}
	return n
}

func secondsToDuration(sec float64) time.Duration {
	if sec <= 0 {
		return 0
	}
	return time.Duration(sec * float64(time.Second))
}
