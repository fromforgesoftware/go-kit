package factory

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

// Builder is a constructor function registered in Builders.
type Builder[P, T any] func(params P) (T, error)

// Builders stores constructor functions keyed by string. Use when the
// produced value depends on caller-supplied parameters.
type Builders[P, T any] struct {
	mu     sync.RWMutex
	data   map[Key]Builder[P, T]
	frozen atomic.Bool
}

// NewBuilders constructs an empty Builders registry.
func NewBuilders[P, T any]() *Builders[P, T] {
	return &Builders[P, T]{data: map[Key]Builder[P, T]{}}
}

// Register stores fn under key.
func (b *Builders[P, T]) Register(key Key, fn Builder[P, T]) error {
	if b.frozen.Load() {
		return ErrFrozen
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.data[key]; exists {
		return fmt.Errorf("%w: %q", ErrDuplicate, key)
	}
	b.data[key] = fn
	return nil
}

// MustRegister panics on error.
func (b *Builders[P, T]) MustRegister(key Key, fn Builder[P, T]) {
	if err := b.Register(key, fn); err != nil {
		panic(err)
	}
}

// Build invokes the registered builder for key with params.
func (b *Builders[P, T]) Build(key Key, params P) (T, error) {
	fn, ok := b.lookup(key)
	if !ok {
		var zero T
		return zero, fmt.Errorf("%w: %q", ErrNotFound, key)
	}
	return fn(params)
}

// Has reports whether key is registered.
func (b *Builders[P, T]) Has(key Key) bool {
	_, ok := b.lookup(key)
	return ok
}

// Keys returns the registered keys, sorted ascending.
func (b *Builders[P, T]) Keys() []Key {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]Key, 0, len(b.data))
	for k := range b.data {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Freeze disables further registration.
func (b *Builders[P, T]) Freeze() { b.frozen.Store(true) }

func (b *Builders[P, T]) lookup(key Key) (Builder[P, T], bool) {
	if b.frozen.Load() {
		fn, ok := b.data[key]
		return fn, ok
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	fn, ok := b.data[key]
	return fn, ok
}
