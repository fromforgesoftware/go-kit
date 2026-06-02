package cache_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/cache"
)

func TestMemory_SetGetMiss(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemory[int]()
	require.NoError(t, c.Set(ctx, "k", 42, 0))

	v, err := c.Get(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, 42, v)

	_, err = c.Get(ctx, "missing")
	assert.ErrorIs(t, err, cache.ErrMiss)
}

func TestMemory_TTLExpires(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemory[string]()
	clock := time.Now()
	c.SetClock(func() time.Time { return clock })

	require.NoError(t, c.Set(ctx, "k", "v", 100*time.Millisecond))
	v, err := c.Get(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, "v", v)

	clock = clock.Add(101 * time.Millisecond)
	_, err = c.Get(ctx, "k")
	assert.ErrorIs(t, err, cache.ErrMiss)
}

func TestMemory_TTLZeroNeverExpires(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemory[int]()
	clock := time.Now()
	c.SetClock(func() time.Time { return clock })

	require.NoError(t, c.Set(ctx, "k", 1, 0))
	clock = clock.Add(24 * time.Hour)

	v, err := c.Get(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, 1, v)
}

func TestMemory_DeleteAndClear(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemory[int]()
	_ = c.Set(ctx, "a", 1, 0)
	_ = c.Set(ctx, "b", 2, 0)

	require.NoError(t, c.Delete(ctx, "a"))
	_, err := c.Get(ctx, "a")
	assert.ErrorIs(t, err, cache.ErrMiss)
	assert.Equal(t, 1, c.Len())

	require.NoError(t, c.Clear(ctx))
	assert.Equal(t, 0, c.Len())
}

func TestMemory_GetOrLoadCachesAfterMiss(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemory[string]()

	calls := 0
	loader := func(context.Context) (string, error) {
		calls++
		return "loaded", nil
	}

	v, err := c.GetOrLoad(ctx, "k", time.Minute, loader)
	require.NoError(t, err)
	assert.Equal(t, "loaded", v)
	assert.Equal(t, 1, calls)

	v, err = c.GetOrLoad(ctx, "k", time.Minute, loader)
	require.NoError(t, err)
	assert.Equal(t, "loaded", v)
	assert.Equal(t, 1, calls, "second call should hit the cache")
}

func TestMemory_GetOrLoadPropagatesError(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemory[string]()

	want := errors.New("boom")
	_, err := c.GetOrLoad(ctx, "k", time.Minute, func(context.Context) (string, error) {
		return "", want
	})
	assert.ErrorIs(t, err, want)

	// Failure must NOT cache the empty value.
	_, err = c.Get(ctx, "k")
	assert.ErrorIs(t, err, cache.ErrMiss)
}

func TestMemory_GetOrLoadCoalescesConcurrentLoaders(t *testing.T) {
	ctx := context.Background()
	c := cache.NewMemory[int]()

	var loaderCalls atomic.Int32
	start := make(chan struct{})
	loader := func(context.Context) (int, error) {
		loaderCalls.Add(1)
		<-start
		return 1, nil
	}

	const n = 50
	var (
		wg      sync.WaitGroup
		results = make([]int, n)
	)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, _ := c.GetOrLoad(ctx, "k", time.Minute, loader)
			results[i] = v
		}(i)
	}

	time.Sleep(10 * time.Millisecond) // let all goroutines reach the inflight check
	close(start)
	wg.Wait()

	assert.Equal(t, int32(1), loaderCalls.Load(), "only one loader should run for concurrent GetOrLoad")
	for _, v := range results {
		assert.Equal(t, 1, v)
	}
}

func TestMemory_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := cache.NewMemory[int]()

	_, err := c.Get(ctx, "k")
	assert.ErrorIs(t, err, context.Canceled)

	err = c.Set(ctx, "k", 1, 0)
	assert.ErrorIs(t, err, context.Canceled)
}
