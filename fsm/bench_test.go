package fsm_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/fsm"
)

func BenchmarkSendTransition(b *testing.B) {
	m := fsm.New(doorSpec())
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Send(ctx, evOpen)
		_ = m.Send(ctx, evClose)
	}
}

func BenchmarkSendIllegal(b *testing.B) {
	m := fsm.New(doorSpec())
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Send(ctx, evUnlock) // illegal from closed
	}
}
