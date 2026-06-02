package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/ratelimit"
)

// BenchmarkAllow measures the hot path: a single keyed take against a warm
// bucket. Run with -benchmem to confirm it stays allocation-free.
func BenchmarkAllow(b *testing.B) {
	s := ratelimit.NewInMemoryStore()
	p := ratelimit.Policy{Limit: 1_000_000_000, Window: time.Second, Burst: 1_000_000_000}
	ctx := context.Background()
	now := time.Unix(100, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.Take(ctx, "k", p, 1, now)
	}
}
