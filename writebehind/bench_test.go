package writebehind_test

import (
	"context"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/writebehind"
)

func BenchmarkPushNonBlocking(b *testing.B) {
	q, _ := writebehind.New(writebehind.Config[int]{
		Capacity:      1024,
		BatchSize:     128,
		FlushInterval: time.Hour, // never auto-flush
		Policy:        writebehind.DropOldest,
		Flusher:       writebehind.FlusherFunc[int](func(_ context.Context, _ []int) error { return nil }),
	})
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = q.Push(ctx, i)
	}
}
