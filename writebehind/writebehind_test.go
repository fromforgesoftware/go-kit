package writebehind_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/writebehind"
)

type capturingFlusher struct {
	mu    sync.Mutex
	calls [][]int
}

func (c *capturingFlusher) Flush(_ context.Context, batch []int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]int, len(batch))
	copy(cp, batch)
	c.calls = append(c.calls, cp)
	return nil
}

func (c *capturingFlusher) total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, b := range c.calls {
		n += len(b)
	}
	return n
}

func TestPushFlushHappyPath(t *testing.T) {
	f := &capturingFlusher{}
	q, err := writebehind.New(writebehind.Config[int]{
		Capacity: 16, BatchSize: 4, FlushInterval: 50 * time.Millisecond, Flusher: f,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- q.Run(ctx) }()

	for i := 0; i < 10; i++ {
		require.NoError(t, q.Push(ctx, i))
	}

	// Wait briefly for batches to drain.
	deadline := time.Now().Add(2 * time.Second)
	for f.total() < 10 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	assert.Equal(t, 10, f.total())
}

func TestDropOldestUnderOverload(t *testing.T) {
	var dropped atomic.Int32
	f := &capturingFlusher{}
	q, err := writebehind.New(writebehind.Config[int]{
		Capacity: 4, BatchSize: 100, FlushInterval: time.Hour, // never auto-flush
		Policy:  writebehind.DropOldest,
		Flusher: f,
		OnDrop:  func(_ int, _ string) { dropped.Add(1) },
	})
	require.NoError(t, err)
	// Don't Run — just exercise the queueing semantics.
	for i := 0; i < 10; i++ {
		require.NoError(t, q.Push(context.Background(), i))
	}
	assert.Equal(t, 4, q.Depth())
	assert.Equal(t, int32(6), dropped.Load())
}

func TestDropNewest(t *testing.T) {
	var dropped atomic.Int32
	q, err := writebehind.New(writebehind.Config[int]{
		Capacity: 2, FlushInterval: time.Hour,
		Policy:  writebehind.DropNewest,
		Flusher: writebehind.FlusherFunc[int](func(_ context.Context, _ []int) error { return nil }),
		OnDrop:  func(_ int, _ string) { dropped.Add(1) },
	})
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		require.NoError(t, q.Push(context.Background(), i))
	}
	assert.Equal(t, 2, q.Depth())
	assert.Equal(t, int32(3), dropped.Load())
}

func TestCoalesceByKeyOverwrites(t *testing.T) {
	type op struct {
		ID  string
		Val int
	}
	// Test the coalesce semantics directly against the queue (no Run);
	// this isolates the "duplicate-key in-queue replaces value"
	// behaviour without the drainer racing pushes.
	q, err := writebehind.New(writebehind.Config[op]{
		Capacity: 8, BatchSize: 100, FlushInterval: time.Hour,
		Policy:  writebehind.CoalesceByKey,
		KeyFunc: func(o op) string { return o.ID },
		Flusher: writebehind.FlusherFunc[op](func(_ context.Context, _ []op) error { return nil }),
	})
	require.NoError(t, err)

	for i := 0; i < 100; i++ {
		require.NoError(t, q.Push(context.Background(), op{ID: strconv.Itoa(i % 3), Val: i}))
	}
	// Three distinct keys → queue depth caps at 3 once steady-state.
	assert.Equal(t, 3, q.Depth())
}

func TestBlockPolicyWaits(t *testing.T) {
	released := make(chan struct{})
	q, err := writebehind.New(writebehind.Config[int]{
		Capacity:      1,
		BatchSize:     1,
		FlushInterval: time.Hour,
		Policy:        writebehind.Block,
		Flusher: writebehind.FlusherFunc[int](func(_ context.Context, _ []int) error {
			<-released // hold the slot indefinitely on first batch
			return nil
		}),
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- q.Run(ctx) }()

	// First Push fills the queue, Run pulls it, calls Flush which blocks.
	require.NoError(t, q.Push(ctx, 1))

	// Second Push fills queue again immediately; third should block.
	require.NoError(t, q.Push(ctx, 2))

	pushCtx, pushCancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer pushCancel()
	err = q.Push(pushCtx, 3)
	assert.ErrorIs(t, err, writebehind.ErrPushBlockTimeout)

	close(released)
	cancel()
	<-done
}

func TestRetryThenExhaust(t *testing.T) {
	var attempts atomic.Int32
	var dropped atomic.Int32
	q, err := writebehind.New(writebehind.Config[int]{
		Capacity:      8,
		BatchSize:     100,
		FlushInterval: 20 * time.Millisecond,
		MaxRetries:    2,
		RetryBackoff:  func(_ int) time.Duration { return 10 * time.Millisecond },
		Flusher: writebehind.FlusherFunc[int](func(_ context.Context, _ []int) error {
			attempts.Add(1)
			return errors.New("network down")
		}),
		OnDrop: func(_ int, reason string) {
			if reason == "retries_exhausted" {
				dropped.Add(1)
			}
		},
	})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- q.Run(ctx) }()

	require.NoError(t, q.Push(ctx, 1))

	// Give retries time to run + exhaust.
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done
	assert.GreaterOrEqual(t, attempts.Load(), int32(3))
	assert.Equal(t, int32(1), dropped.Load())
}

func TestRunFinalDrainOnShutdown(t *testing.T) {
	f := &capturingFlusher{}
	q, err := writebehind.New(writebehind.Config[int]{
		Capacity:      8,
		BatchSize:     100,
		FlushInterval: time.Hour, // never periodic-flush so the final drain is the only chance
		Flusher:       f,
	})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- q.Run(ctx) }()

	for i := 0; i < 5; i++ {
		require.NoError(t, q.Push(ctx, i))
	}
	// Cancel before any periodic flush has fired.
	cancel()
	<-done
	assert.Equal(t, 5, f.total(), "final drain must flush remaining items")
}

func TestConfigValidation(t *testing.T) {
	_, err := writebehind.New(writebehind.Config[int]{Flusher: writebehind.FlusherFunc[int](func(_ context.Context, _ []int) error { return nil })})
	assert.ErrorIs(t, err, writebehind.ErrCapacityRequired)

	_, err = writebehind.New(writebehind.Config[int]{Capacity: 1})
	assert.ErrorIs(t, err, writebehind.ErrFlusherRequired)

	_, err = writebehind.New(writebehind.Config[int]{
		Capacity: 1, Policy: writebehind.CoalesceByKey,
		Flusher: writebehind.FlusherFunc[int](func(_ context.Context, _ []int) error { return nil }),
	})
	assert.ErrorIs(t, err, writebehind.ErrCoalesceNeedsKey)
}
