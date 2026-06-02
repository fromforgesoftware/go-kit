package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/cache"
)

// BenchmarkMemory_Get hits the inmem backend in steady state —
// representative of the L1 path inside a Tiered cache. Lock cost
// dominates; this catches regressions in the locking strategy.
func BenchmarkMemory_Get(b *testing.B) {
	c := cache.NewMemory[pet]()
	_ = c.Set(context.Background(), "rex", pet{ID: "1", Name: "Rex"}, time.Hour)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(context.Background(), "rex")
	}
}

// BenchmarkMemory_SetGet alternates Set + Get to cover write + read
// paths under realistic mixed load.
func BenchmarkMemory_SetGet(b *testing.B) {
	c := cache.NewMemory[pet]()
	val := pet{ID: "1", Name: "Rex"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Set(context.Background(), "rex", val, time.Hour)
		_, _ = c.Get(context.Background(), "rex")
	}
}

// BenchmarkNamespaced_Get measures the prefix-string concatenation
// overhead of the Namespaced wrapper. Trivial in absolute terms but
// the inner loop on hot caches.
func BenchmarkNamespaced_Get(b *testing.B) {
	c := cache.NewNamespaced[pet](cache.NewMemory[pet](), "petstore")
	_ = c.Set(context.Background(), "rex", pet{ID: "1", Name: "Rex"}, time.Hour)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(context.Background(), "rex")
	}
}
