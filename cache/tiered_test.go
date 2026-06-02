package cache_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/cache"
)

func TestTiered_GetHitsL1First(t *testing.T) {
	l1 := cache.NewMemory[pet]()
	l2 := cache.NewMemory[pet]()
	tier := cache.NewTiered[pet](l1, l2)

	require.NoError(t, l1.Set(context.Background(), "rex", pet{ID: "L1"}, time.Minute))
	require.NoError(t, l2.Set(context.Background(), "rex", pet{ID: "L2"}, time.Minute))

	got, err := tier.Get(context.Background(), "rex")
	require.NoError(t, err)
	assert.Equal(t, "L1", got.ID, "L1 should win when both have the key")
}

func TestTiered_GetWarmsL1OnL2Hit(t *testing.T) {
	l1 := cache.NewMemory[pet]()
	l2 := cache.NewMemory[pet]()
	tier := cache.NewTiered[pet](l1, l2)

	require.NoError(t, l2.Set(context.Background(), "rex", pet{ID: "from-L2"}, time.Minute))

	_, err := tier.Get(context.Background(), "rex")
	require.NoError(t, err)

	// L1 should now have the entry too.
	got, err := l1.Get(context.Background(), "rex")
	require.NoError(t, err)
	assert.Equal(t, "from-L2", got.ID)
}

func TestTiered_SetWritesBoth(t *testing.T) {
	l1 := cache.NewMemory[pet]()
	l2 := cache.NewMemory[pet]()
	tier := cache.NewTiered[pet](l1, l2)

	require.NoError(t, tier.Set(context.Background(), "rex", pet{ID: "1"}, time.Minute))

	g1, err := l1.Get(context.Background(), "rex")
	require.NoError(t, err)
	g2, err := l2.Get(context.Background(), "rex")
	require.NoError(t, err)
	assert.Equal(t, "1", g1.ID)
	assert.Equal(t, "1", g2.ID)
}

func TestTiered_DeleteRemovesFromBoth(t *testing.T) {
	l1 := cache.NewMemory[pet]()
	l2 := cache.NewMemory[pet]()
	tier := cache.NewTiered[pet](l1, l2)

	require.NoError(t, tier.Set(context.Background(), "rex", pet{ID: "1"}, time.Minute))
	require.NoError(t, tier.Delete(context.Background(), "rex"))

	_, err := l1.Get(context.Background(), "rex")
	assert.ErrorIs(t, err, cache.ErrMiss)
	_, err = l2.Get(context.Background(), "rex")
	assert.ErrorIs(t, err, cache.ErrMiss)
}

func TestTiered_GetOrLoadFillsBoth(t *testing.T) {
	l1 := cache.NewMemory[pet]()
	l2 := cache.NewMemory[pet]()
	tier := cache.NewTiered[pet](l1, l2)

	var calls atomic.Int32
	loader := func(_ context.Context) (pet, error) {
		calls.Add(1)
		return pet{ID: "fresh"}, nil
	}

	got, err := tier.GetOrLoad(context.Background(), "rex", time.Minute, loader)
	require.NoError(t, err)
	assert.Equal(t, "fresh", got.ID)
	assert.Equal(t, int32(1), calls.Load())

	// Both tiers should be warm now; loader shouldn't fire again.
	got, err = tier.GetOrLoad(context.Background(), "rex", time.Minute, loader)
	require.NoError(t, err)
	assert.Equal(t, "fresh", got.ID)
	assert.Equal(t, int32(1), calls.Load(), "second GetOrLoad served from cache")
}

func TestTiered_L1ErrorHookFires(t *testing.T) {
	l1 := &erroringCache{}
	l2 := cache.NewMemory[pet]()
	tier := cache.NewTiered[pet](l1, l2)
	var ops []string
	tier.OnL1Error = func(op string, _ error) { ops = append(ops, op) }

	require.NoError(t, tier.Set(context.Background(), "k", pet{}, time.Minute))
	assert.Contains(t, ops, "set")
}

// erroringCache is a Cache[pet] whose every operation fails — used
// to exercise the OnL1Error hook in Tiered.
type erroringCache struct{}

func (erroringCache) Get(_ context.Context, _ string) (pet, error) {
	return pet{}, errors.New("boom")
}
func (erroringCache) Set(_ context.Context, _ string, _ pet, _ time.Duration) error {
	return errors.New("boom")
}
func (erroringCache) Delete(_ context.Context, _ string) error { return errors.New("boom") }
func (erroringCache) GetOrLoad(_ context.Context, _ string, _ time.Duration, _ func(ctx context.Context) (pet, error)) (pet, error) {
	return pet{}, errors.New("boom")
}
func (erroringCache) Clear(_ context.Context) error { return errors.New("boom") }
