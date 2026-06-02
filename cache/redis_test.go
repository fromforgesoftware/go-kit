package cache_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/cache"
)

type pet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func newMiniredisClient(t *testing.T) (*miniredis.Miniredis, redis.UniversalClient) {
	t.Helper()
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	return mr, c
}

func TestRedis_GetSetRoundTrip(t *testing.T) {
	_, c := newMiniredisClient(t)
	r := cache.NewRedis[pet](c, cache.WithKeyPrefix[pet]("petstore"))

	require.NoError(t, r.Set(context.Background(), "rex", pet{ID: "1", Name: "Rex"}, 0))
	got, err := r.Get(context.Background(), "rex")
	require.NoError(t, err)
	assert.Equal(t, pet{ID: "1", Name: "Rex"}, got)
}

func TestRedis_GetMiss(t *testing.T) {
	_, c := newMiniredisClient(t)
	r := cache.NewRedis[pet](c)
	_, err := r.Get(context.Background(), "nope")
	assert.ErrorIs(t, err, cache.ErrMiss)
}

func TestRedis_TTLExpiry(t *testing.T) {
	mr, c := newMiniredisClient(t)
	r := cache.NewRedis[pet](c)
	require.NoError(t, r.Set(context.Background(), "k", pet{ID: "x"}, 100*time.Millisecond))

	mr.FastForward(200 * time.Millisecond)
	_, err := r.Get(context.Background(), "k")
	assert.ErrorIs(t, err, cache.ErrMiss)
}

func TestRedis_Delete(t *testing.T) {
	_, c := newMiniredisClient(t)
	r := cache.NewRedis[pet](c)
	require.NoError(t, r.Set(context.Background(), "k", pet{ID: "x"}, 0))
	require.NoError(t, r.Delete(context.Background(), "k"))
	_, err := r.Get(context.Background(), "k")
	assert.ErrorIs(t, err, cache.ErrMiss)
}

func TestRedis_GetOrLoadCoalesces(t *testing.T) {
	_, c := newMiniredisClient(t)
	r := cache.NewRedis[pet](c)

	var calls int
	var mu sync.Mutex
	loader := func(_ context.Context) (pet, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		return pet{ID: "1", Name: "Rex"}, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.GetOrLoad(context.Background(), "rex", time.Minute, loader)
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, calls, "concurrent GetOrLoad calls should coalesce to a single loader run")
}

func TestRedis_ClearRefusesWithoutPrefix(t *testing.T) {
	_, c := newMiniredisClient(t)
	r := cache.NewRedis[pet](c)
	err := r.Clear(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WithKeyPrefix")
}

func TestRedis_ClearScopedToPrefix(t *testing.T) {
	_, c := newMiniredisClient(t)
	r := cache.NewRedis[pet](c, cache.WithKeyPrefix[pet]("petstore"))

	require.NoError(t, r.Set(context.Background(), "a", pet{ID: "1"}, 0))
	require.NoError(t, r.Set(context.Background(), "b", pet{ID: "2"}, 0))
	// Plant a key outside the prefix; Clear must not touch it.
	require.NoError(t, c.Set(context.Background(), "other:c", "x", 0).Err())

	require.NoError(t, r.Clear(context.Background()))

	_, err := r.Get(context.Background(), "a")
	assert.ErrorIs(t, err, cache.ErrMiss)
	v, err := c.Get(context.Background(), "other:c").Result()
	require.NoError(t, err)
	assert.Equal(t, "x", v, "Clear must not touch keys outside the configured prefix")
}

// JSONCodec is the default; ensure decode failures surface cleanly.
func TestRedis_DecodeErrorSurfaces(t *testing.T) {
	_, c := newMiniredisClient(t)
	r := cache.NewRedis[pet](c)
	require.NoError(t, c.Set(context.Background(), "k", "not-json", 0).Err())
	_, err := r.Get(context.Background(), "k")
	require.Error(t, err)
	// Just assert it's not the miss sentinel — the underlying error
	// is a json.SyntaxError.
	assert.False(t, errors.Is(err, cache.ErrMiss))
}
