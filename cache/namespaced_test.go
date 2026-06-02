package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/cache"
)

func TestNamespaced_PrefixesKeys(t *testing.T) {
	inner := cache.NewMemory[pet]()
	a := cache.NewNamespaced[pet](inner, "svcA")
	b := cache.NewNamespaced[pet](inner, "svcB")

	require.NoError(t, a.Set(context.Background(), "rex", pet{ID: "A"}, time.Minute))
	require.NoError(t, b.Set(context.Background(), "rex", pet{ID: "B"}, time.Minute))

	gotA, err := a.Get(context.Background(), "rex")
	require.NoError(t, err)
	gotB, err := b.Get(context.Background(), "rex")
	require.NoError(t, err)
	assert.Equal(t, "A", gotA.ID)
	assert.Equal(t, "B", gotB.ID)
}

func TestNamespaced_AddsTrailingColon(t *testing.T) {
	inner := cache.NewMemory[pet]()
	// "svc" without trailing colon should still produce "svc:rex".
	n := cache.NewNamespaced[pet](inner, "svc")
	require.NoError(t, n.Set(context.Background(), "rex", pet{ID: "1"}, time.Minute))

	// Verify the underlying key includes the colon.
	got, err := inner.Get(context.Background(), "svc:rex")
	require.NoError(t, err)
	assert.Equal(t, "1", got.ID)
}
