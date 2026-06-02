package fsm_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/fsm"
)

const (
	aClosed fsm.AtomicState = iota
	aOpen
	aLocked
)

const (
	aEvOpen fsm.AtomicEvent = iota
	aEvClose
	aEvLock
)

func atomicDoorSpec() fsm.AtomicSpec {
	return fsm.AtomicSpec{
		Initial: aClosed,
		Transitions: []fsm.AtomicTransition{
			{From: aClosed, Event: aEvOpen, To: aOpen},
			{From: aOpen, Event: aEvClose, To: aClosed},
			{From: aClosed, Event: aEvLock, To: aLocked},
		},
	}
}

func TestAtomicBasicTransitions(t *testing.T) {
	m := fsm.NewAtomic(atomicDoorSpec())
	assert.Equal(t, aClosed, m.State())
	require.NoError(t, m.Send(context.Background(), aEvOpen))
	assert.Equal(t, aOpen, m.State())
	require.NoError(t, m.Send(context.Background(), aEvClose))
	assert.True(t, m.IsState(aClosed))
}

func TestAtomicConcurrentReads(t *testing.T) {
	m := fsm.NewAtomic(atomicDoorSpec())
	var wg sync.WaitGroup
	var reads atomic.Uint64
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10_000; j++ {
				_ = m.State()
				reads.Add(1)
			}
		}()
	}
	// Single writer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx := context.Background()
		for i := 0; i < 1000; i++ {
			_ = m.Send(ctx, aEvOpen)
			_ = m.Send(ctx, aEvClose)
		}
	}()
	wg.Wait()
	assert.Equal(t, uint64(80_000), reads.Load())
}

func TestAtomicIllegalTransition(t *testing.T) {
	m := fsm.NewAtomic(atomicDoorSpec())
	err := m.Send(context.Background(), aEvClose) // illegal from closed
	assert.ErrorIs(t, err, fsm.ErrIllegalTransition)
}

func BenchmarkAtomicSendTransition(b *testing.B) {
	m := fsm.NewAtomic(atomicDoorSpec())
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Send(ctx, aEvOpen)
		_ = m.Send(ctx, aEvClose)
	}
}

func BenchmarkAtomicStateLoad(b *testing.B) {
	m := fsm.NewAtomic(atomicDoorSpec())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.State()
	}
}
