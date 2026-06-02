package scheduler_test

import (
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/scheduler"
)

// BenchmarkEventMapIncrementalUpdate is the realistic game-server pattern:
// a 1k steady-state queue where each Update drains only the few events
// that are due, then refills. Heap is O(k log n) — orders of magnitude
// faster than re-sorting per Update.
func BenchmarkEventMapIncrementalUpdate(b *testing.B) {
	clk := scheduler.NewMockClock(time.Unix(0, 0))
	m := scheduler.NewEventMap(clk)
	for i := 0; i < 1000; i++ {
		m.Schedule(scheduler.EventID(i), time.Duration(i*7)*time.Millisecond, 0)
	}
	due := make([]scheduler.EventID, 0, 16)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clk.Add(time.Millisecond)
		m.Update(clk.Now(), &due)
		for _, id := range due {
			m.Schedule(id, time.Second, 0)
		}
	}
}

// BenchmarkEventMap100kBulkDrain stresses the worst case for a heap: a
// 100k queue drained entirely in one Update. Heap pops are O(log n) each,
// totalling O(n log n); a sort-and-drain achieves O(n) on already-sorted
// input. Real workloads do not look like this — see Incremental above.
func BenchmarkEventMap100kBulkDrain(b *testing.B) {
	clk := scheduler.NewMockClock(time.Unix(0, 0))
	target := time.Unix(0, 0).Add(200 * time.Millisecond)
	due := make([]scheduler.EventID, 0, 100_000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		m := scheduler.NewEventMap(clk)
		for j := 0; j < 100_000; j++ {
			m.Schedule(scheduler.EventID(j), time.Duration(j)*time.Microsecond, 0)
		}
		b.StartTimer()
		m.Update(target, &due)
	}
}

func BenchmarkEventMapSchedule(b *testing.B) {
	clk := scheduler.NewMockClock(time.Unix(0, 0))
	m := scheduler.NewEventMap(clk)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Schedule(scheduler.EventID(i), time.Microsecond, 0)
	}
}
