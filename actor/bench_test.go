package actor_test

import (
	"testing"

	"github.com/fromforgesoftware/go-kit/actor"
)

func BenchmarkMailboxTrySendDrain(b *testing.B) {
	mb := actor.NewMailbox[int](1024)
	buf := make([]int, 0, 1024)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 1024; j++ {
			_ = mb.TrySend(j)
		}
		mb.Drain(&buf)
	}
}

func BenchmarkMailboxDrainEmpty(b *testing.B) {
	mb := actor.NewMailbox[int](1024)
	buf := make([]int, 0, 64)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mb.Drain(&buf)
	}
}
