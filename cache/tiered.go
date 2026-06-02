package cache

import (
	"context"
	"errors"
	"time"
)

// Tiered chains two caches: L1 (fast, local) → L2 (shared, remote).
// Reads probe L1 first; on miss they probe L2 and warm L1. Writes
// fan out to both. Failures in L2 surface; failures in L1 are
// non-fatal and logged via the optional OnL1Error hook.
//
// Typical setup: inmem L1 + redis L2 — sub-µs reads when warm,
// shared invalidation across pods when cold.
type Tiered[V any] struct {
	l1 Cache[V]
	l2 Cache[V]

	// OnL1Error is called when an L1 operation fails. Defaults to a
	// no-op; wire to a logger to capture serialization or eviction
	// errors without aborting the call.
	OnL1Error func(op string, err error)

	// L1TTL clamps how long a warmed entry stays in L1. Defaults to
	// the L2 TTL passed to Set/GetOrLoad. Useful when L1 should
	// expire faster than L2 to bound staleness.
	L1TTL time.Duration
}

// NewTiered composes L1 over L2.
func NewTiered[V any](l1, l2 Cache[V]) *Tiered[V] {
	return &Tiered[V]{
		l1:        l1,
		l2:        l2,
		OnL1Error: func(string, error) {},
	}
}

func (t *Tiered[V]) Get(ctx context.Context, key string) (V, error) {
	if v, err := t.l1.Get(ctx, key); err == nil {
		return v, nil
	} else if !errors.Is(err, ErrMiss) {
		t.OnL1Error("get", err)
	}
	v, err := t.l2.Get(ctx, key)
	if err != nil {
		return v, err
	}
	if setErr := t.l1.Set(ctx, key, v, t.l1TTL(0)); setErr != nil {
		t.OnL1Error("set-warm", setErr)
	}
	return v, nil
}

func (t *Tiered[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error {
	if err := t.l1.Set(ctx, key, value, t.l1TTL(ttl)); err != nil {
		t.OnL1Error("set", err)
	}
	return t.l2.Set(ctx, key, value, ttl)
}

func (t *Tiered[V]) Delete(ctx context.Context, key string) error {
	if err := t.l1.Delete(ctx, key); err != nil {
		t.OnL1Error("delete", err)
	}
	return t.l2.Delete(ctx, key)
}

func (t *Tiered[V]) GetOrLoad(
	ctx context.Context,
	key string,
	ttl time.Duration,
	loader func(ctx context.Context) (V, error),
) (V, error) {
	if v, err := t.Get(ctx, key); err == nil {
		return v, nil
	} else if !errors.Is(err, ErrMiss) {
		return v, err
	}
	v, err := loader(ctx)
	if err != nil {
		return v, err
	}
	if setErr := t.Set(ctx, key, v, ttl); setErr != nil {
		return v, setErr
	}
	return v, nil
}

func (t *Tiered[V]) Clear(ctx context.Context) error {
	if err := t.l1.Clear(ctx); err != nil {
		t.OnL1Error("clear", err)
	}
	return t.l2.Clear(ctx)
}

func (t *Tiered[V]) l1TTL(l2TTL time.Duration) time.Duration {
	if t.L1TTL > 0 && (l2TTL == 0 || t.L1TTL < l2TTL) {
		return t.L1TTL
	}
	return l2TTL
}
