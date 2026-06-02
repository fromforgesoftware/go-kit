package factory

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

// Key is the identifier under which entries are stored.
type Key = string

// Errors surfaced by the registries.
var (
	ErrDuplicate = errors.New("factory: key already registered")
	ErrFrozen    = errors.New("factory: registry is frozen")
	ErrNotFound  = errors.New("factory: key not found")
)

// Registry stores values of T keyed by string.
type Registry[T any] struct {
	mu     sync.RWMutex
	data   map[Key]T
	frozen atomic.Bool
}

// New constructs an empty Registry.
func New[T any]() *Registry[T] {
	return &Registry[T]{data: map[Key]T{}}
}

// Register stores value under key. Returns ErrDuplicate if key is
// already present, ErrFrozen if the registry has been frozen.
func (r *Registry[T]) Register(key Key, value T) error {
	if r.frozen.Load() {
		return ErrFrozen
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.data[key]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicate, key)
	}
	r.data[key] = value
	return nil
}

// MustRegister panics on error. For init-time use.
func (r *Registry[T]) MustRegister(key Key, value T) {
	if err := r.Register(key, value); err != nil {
		panic(err)
	}
}

// Get returns the value under key.
func (r *Registry[T]) Get(key Key) (T, bool) {
	if r.frozen.Load() {
		// Lock-free path — map is immutable after Freeze.
		v, ok := r.data[key]
		return v, ok
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.data[key]
	return v, ok
}

// Keys returns the registered keys, sorted ascending.
func (r *Registry[T]) Keys() []Key {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Key, 0, len(r.data))
	for k := range r.data {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Freeze disables further registration; subsequent Get calls bypass
// the lock entirely. Idempotent.
func (r *Registry[T]) Freeze() { r.frozen.Store(true) }

// IsFrozen reports the freeze flag.
func (r *Registry[T]) IsFrozen() bool { return r.frozen.Load() }
