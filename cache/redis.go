package cache

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis is a Cache[V] backed by go-redis. Values are JSON-encoded by
// default; supply WithCodec to pick a different format.
//
// Concurrent loaders for the same key are coalesced in-process via a
// per-instance singleflight map. Cross-instance coalescing would
// need a Redis lock (SETNX) — out of scope for the kit-level
// abstraction; layer it on top when you need it.
type Redis[V any] struct {
	client    redis.UniversalClient
	codec     Codec[V]
	keyPrefix string

	mu       sync.Mutex
	inFlight map[string]*inflight[V]
}

// Codec converts V to/from bytes for storage.
type Codec[V any] interface {
	Encode(V) ([]byte, error)
	Decode([]byte) (V, error)
}

// JSONCodec is the default Codec — encodes V via encoding/json.
type JSONCodec[V any] struct{}

func (JSONCodec[V]) Encode(v V) ([]byte, error) { return json.Marshal(v) }
func (JSONCodec[V]) Decode(b []byte) (V, error) {
	var v V
	err := json.Unmarshal(b, &v)
	return v, err
}

// RedisOption configures a Redis cache.
type RedisOption[V any] func(*Redis[V])

// WithCodec swaps the default JSON codec for a custom one (msgpack,
// protobuf, …).
func WithCodec[V any](c Codec[V]) RedisOption[V] {
	return func(r *Redis[V]) { r.codec = c }
}

// WithKeyPrefix prepends prefix to every key. The trailing colon is
// added if missing.
func WithKeyPrefix[V any](prefix string) RedisOption[V] {
	return func(r *Redis[V]) { r.keyPrefix = ensureColon(prefix) }
}

// NewRedis returns a Cache[V] backed by client. The client is
// expected to be already configured (URL, auth, pool size); the
// cache doesn't own its lifecycle.
func NewRedis[V any](client redis.UniversalClient, opts ...RedisOption[V]) *Redis[V] {
	r := &Redis[V]{
		client:   client,
		codec:    JSONCodec[V]{},
		inFlight: map[string]*inflight[V]{},
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

func (r *Redis[V]) Get(ctx context.Context, key string) (V, error) {
	var zero V
	b, err := r.client.Get(ctx, r.keyPrefix+key).Bytes()
	if errors.Is(err, redis.Nil) {
		return zero, ErrMiss
	}
	if err != nil {
		return zero, err
	}
	return r.codec.Decode(b)
}

func (r *Redis[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error {
	b, err := r.codec.Encode(value)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, r.keyPrefix+key, b, ttl).Err()
}

func (r *Redis[V]) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, r.keyPrefix+key).Err()
}

func (r *Redis[V]) GetOrLoad(
	ctx context.Context,
	key string,
	ttl time.Duration,
	loader func(ctx context.Context) (V, error),
) (V, error) {
	if v, err := r.Get(ctx, key); err == nil {
		return v, nil
	} else if !errors.Is(err, ErrMiss) {
		return v, err
	}

	// In-process singleflight. Cross-instance coalescing would need
	// a Redis SETNX-based lock; left to the caller when needed.
	r.mu.Lock()
	if w, ok := r.inFlight[key]; ok {
		r.mu.Unlock()
		w.wg.Wait()
		return w.value, w.err
	}
	w := &inflight[V]{}
	w.wg.Add(1)
	r.inFlight[key] = w
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.inFlight, key)
		r.mu.Unlock()
		w.wg.Done()
	}()

	v, err := loader(ctx)
	if err != nil {
		w.err = err
		return v, err
	}
	if setErr := r.Set(ctx, key, v, ttl); setErr != nil {
		w.err = setErr
		return v, setErr
	}
	w.value = v
	return v, nil
}

// Clear removes every key under the configured prefix. Implemented
// via SCAN + DEL in batches; safe to call on a live cache but O(n)
// in keyspace size. No prefix configured means it's a no-op (we
// refuse to FLUSHDB a shared Redis).
func (r *Redis[V]) Clear(ctx context.Context) error {
	if r.keyPrefix == "" {
		return errors.New("cache: Redis.Clear requires WithKeyPrefix to avoid flushing the whole DB")
	}
	pattern := r.keyPrefix + "*"
	iter := r.client.Scan(ctx, 0, pattern, 500).Iterator()
	var batch []string
	for iter.Next(ctx) {
		batch = append(batch, iter.Val())
		if len(batch) >= 500 {
			if err := r.client.Del(ctx, batch...).Err(); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(batch) > 0 {
		return r.client.Del(ctx, batch...).Err()
	}
	return nil
}

func ensureColon(s string) string {
	if s == "" || s[len(s)-1] == ':' {
		return s
	}
	return s + ":"
}
