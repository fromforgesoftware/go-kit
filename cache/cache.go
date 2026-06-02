// Package cache is the kit's transport-agnostic key/value cache abstraction.
//
// Implementations:
//   - NewMemory[V]() — in-process LRU-ish cache with TTL eviction
//   - (future) NewRedis[V](client, prefix) — Redis-backed adapter
//
// Consumers code against Cache[V], pick an implementation at wiring time, and
// can swap memory ↔ Redis without touching call sites.
package cache

import (
	"context"
	"errors"
	"time"
)

// ErrMiss is returned by Get when the key is absent or has expired.
var ErrMiss = errors.New("cache: miss")

// Cache is the common interface backed by every implementation.
type Cache[V any] interface {
	Get(ctx context.Context, key string) (V, error)
	Set(ctx context.Context, key string, value V, ttl time.Duration) error
	Delete(ctx context.Context, key string) error

	// GetOrLoad fetches the cached value or calls loader on miss, storing the
	// result with ttl before returning. Concurrent loaders for the same key
	// are coalesced: only one loader runs, the rest wait for the result.
	GetOrLoad(ctx context.Context, key string, ttl time.Duration, loader func(ctx context.Context) (V, error)) (V, error)

	// Clear flushes the entire namespace.
	Clear(ctx context.Context) error
}
