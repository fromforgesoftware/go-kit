package shutdown_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/shutdown"
)

func TestCoordinator_RunsHooksInLIFOOnContextCancel(t *testing.T) {
	c := shutdown.New(shutdown.WithGraceWindow(time.Second))
	var (
		mu    sync.Mutex
		order []string
	)
	add := func(name string) shutdown.Hook {
		return func(context.Context) error {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
			return nil
		}
	}
	c.Register("a", add("a"))
	c.Register("b", add("b"))
	c.Register("c", add("c"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, c.Run(ctx))
	assert.Equal(t, []string{"c", "b", "a"}, order)
}

func TestCoordinator_StopTriggersShutdown(t *testing.T) {
	c := shutdown.New()
	var ran atomic.Bool
	c.Register("h", func(context.Context) error {
		ran.Store(true)
		return nil
	})

	go func() {
		time.Sleep(10 * time.Millisecond)
		c.Stop()
	}()

	require.NoError(t, c.Run(context.Background()))
	assert.True(t, ran.Load())
}

func TestCoordinator_StopIsIdempotent(t *testing.T) {
	c := shutdown.New()
	c.Stop()
	c.Stop()
	require.NoError(t, c.Run(context.Background()))
}

func TestCoordinator_FailedHooksAreJoined(t *testing.T) {
	c := shutdown.New(shutdown.WithHookTimeout(50 * time.Millisecond))
	c.Register("ok", func(context.Context) error { return nil })
	c.Register("boom", func(context.Context) error { return errors.New("ouch") })
	c.Register("bad", func(context.Context) error { return errors.New("nope") })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.Run(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ouch")
	assert.Contains(t, err.Error(), "nope")
}

func TestCoordinator_HookTimeoutIsEnforced(t *testing.T) {
	c := shutdown.New(shutdown.WithHookTimeout(20*time.Millisecond), shutdown.WithGraceWindow(time.Second))
	c.Register("slow", func(ctx context.Context) error {
		select {
		case <-time.After(200 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := c.Run(ctx)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 100*time.Millisecond, "must abandon the slow hook")
}

func TestCoordinator_LogsLifecycleEvents(t *testing.T) {
	var (
		mu   sync.Mutex
		logs []string
	)
	c := shutdown.New(shutdown.WithLogger(func(format string, args ...any) {
		mu.Lock()
		logs = append(logs, format)
		mu.Unlock()
	}))
	c.Register("h", func(context.Context) error { return nil })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, c.Run(ctx))
	mu.Lock()
	defer mu.Unlock()
	assert.NotEmpty(t, logs)
}
