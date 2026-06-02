package extractor_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/extractor"
)

func BenchmarkWorker1024Units(b *testing.B) {
	units := make([]int, 1024)
	for i := range units {
		units[i] = i
	}
	w, _ := extractor.NewWorker(extractor.WorkerConfig[int, int]{
		Concurrency: 8,
		Process:     func(_ context.Context, n int) (int, error) { return n + 1, nil },
	})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.Run(context.Background(), units)
	}
}
