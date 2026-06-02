package cache

import (
	"context"
	"time"
)

// Namespaced wraps any Cache[V] with a key prefix so multiple
// services (or multiple subsystems within one service) can share a
// physical Redis without colliding on keys.
//
// The wrapped cache's Clear is delegated as-is — implementations are
// responsible for honouring the prefix internally. The Redis backend
// already does the right thing (SCAN under prefix); Memory's Clear
// flushes everything, which is fine for tests but means a Namespaced
// Memory shares its keyspace across namespaces.
type Namespaced[V any] struct {
	inner  Cache[V]
	prefix string
}

// NewNamespaced returns a wrapper that prepends prefix (followed by
// ":") to every key before delegating.
func NewNamespaced[V any](inner Cache[V], prefix string) *Namespaced[V] {
	return &Namespaced[V]{inner: inner, prefix: ensureColon(prefix)}
}

func (n *Namespaced[V]) Get(ctx context.Context, key string) (V, error) {
	return n.inner.Get(ctx, n.prefix+key)
}

func (n *Namespaced[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error {
	return n.inner.Set(ctx, n.prefix+key, value, ttl)
}

func (n *Namespaced[V]) Delete(ctx context.Context, key string) error {
	return n.inner.Delete(ctx, n.prefix+key)
}

func (n *Namespaced[V]) GetOrLoad(
	ctx context.Context,
	key string,
	ttl time.Duration,
	loader func(ctx context.Context) (V, error),
) (V, error) {
	return n.inner.GetOrLoad(ctx, n.prefix+key, ttl, loader)
}

func (n *Namespaced[V]) Clear(ctx context.Context) error { return n.inner.Clear(ctx) }
