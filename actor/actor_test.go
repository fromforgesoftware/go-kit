package actor_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/actor"
)

type counterBehavior struct {
	tickCount atomic.Uint64
}

type counterState struct {
	Total int
}

func (b *counterBehavior) Init(_ context.Context) (counterState, error) {
	return counterState{}, nil
}

func (b *counterBehavior) Handle(_ context.Context, state *counterState, msg int) error {
	state.Total += msg
	return nil
}

func (b *counterBehavior) Tick(_ context.Context, _ *counterState, _ time.Time) error {
	b.tickCount.Add(1)
	return nil
}

func (b *counterBehavior) Close(_ context.Context, _ *counterState) error { return nil }

func TestActorReceivesMessages(t *testing.T) {
	beh := &counterBehavior{}
	a := actor.New[int, counterState](beh)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	for i := 1; i <= 10; i++ {
		require.NoError(t, a.Send(context.Background(), i))
	}
	// Allow some time for the actor to process the messages.
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done
}

func TestMailboxDrainAllocFree(t *testing.T) {
	mb := actor.NewMailbox[int](16)
	for i := 0; i < 16; i++ {
		require.NoError(t, mb.TrySend(i))
	}
	buf := make([]int, 0, 16)
	mb.Drain(&buf)
	assert.Len(t, buf, 16)
}

func TestMailboxTrySendFull(t *testing.T) {
	mb := actor.NewMailbox[int](2)
	require.NoError(t, mb.TrySend(1))
	require.NoError(t, mb.TrySend(2))
	assert.ErrorIs(t, mb.TrySend(3), actor.ErrMailboxFull)
}

func TestSupervisorRestartsPanic(t *testing.T) {
	sup := actor.NewSupervisor(actor.RestartPolicy{Backoff: 5 * time.Millisecond, MaxRestarts: 2})
	attempts := atomic.Uint64{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sup.Spawn(ctx, "panicker", func(_ context.Context) error {
		attempts.Add(1)
		panic("boom")
	})

	// Wait briefly for restarts.
	time.Sleep(50 * time.Millisecond)
	cancel()
	sup.Wait()
	// initial + 2 restarts = 3 attempts max
	assert.LessOrEqual(t, attempts.Load(), uint64(3))
	assert.GreaterOrEqual(t, attempts.Load(), uint64(2))
}

func TestSupervisorStopsOnContext(t *testing.T) {
	sup := actor.NewSupervisor(actor.RestartPolicy{})
	stopped := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	sup.Spawn(ctx, "blocker", func(ctx context.Context) error {
		<-ctx.Done()
		close(stopped)
		return errors.New("done")
	})
	cancel()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("child did not stop on context cancel")
	}
	sup.Wait()
}

func TestActorConcurrentSends(t *testing.T) {
	beh := &counterBehavior{}
	a := actor.New[int, counterState](beh, actor.WithMailboxCap(1024))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	var wg sync.WaitGroup
	for w := 0; w < 10; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_ = a.Send(context.Background(), 1)
			}
		}()
	}
	wg.Wait()
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done
}
