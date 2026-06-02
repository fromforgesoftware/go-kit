package actor_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/actor"
)

func TestLockFreeMailboxRoundTrip(t *testing.T) {
	mb := actor.NewLockFreeMailbox[int](16)
	for i := 0; i < 8; i++ {
		require.NoError(t, mb.TrySend(i))
	}
	buf := make([]int, 0, 8)
	mb.Drain(&buf)
	assert.Equal(t, []int{0, 1, 2, 3, 4, 5, 6, 7}, buf)
}

func TestLockFreeMailboxFull(t *testing.T) {
	mb := actor.NewLockFreeMailbox[int](4)
	for i := 0; i < 4; i++ {
		require.NoError(t, mb.TrySend(i))
	}
	err := mb.TrySend(99)
	assert.ErrorIs(t, err, actor.ErrMailboxFull)
}

func TestLockFreeMailboxConcurrentProducers(t *testing.T) {
	mb := actor.NewLockFreeMailbox[int](4096)
	const producers = 8
	const perProducer = 500
	var wg sync.WaitGroup
	sent := atomic.Uint64{}
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perProducer; i++ {
				for mb.TrySend(i) == actor.ErrMailboxFull {
					// busy wait — consumer will drain
				}
				sent.Add(1)
			}
		}()
	}
	// Consumer.
	received := []int{}
	buf := make([]int, 0, 128)
	done := make(chan struct{})
	go func() {
		for {
			mb.Drain(&buf)
			received = append(received, buf...)
			if uint64(len(received)) >= producers*perProducer {
				close(done)
				return
			}
		}
	}()
	wg.Wait()
	<-done
	assert.Equal(t, uint64(producers*perProducer), sent.Load())
	assert.Equal(t, producers*perProducer, len(received))
}

func BenchmarkLockFreeMailboxTrySendDrain(b *testing.B) {
	mb := actor.NewLockFreeMailbox[int](1024)
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
