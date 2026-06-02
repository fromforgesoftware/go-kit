package cron_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/cron"
)

func TestScheduler_RunsRegisteredJob(t *testing.T) {
	var calls atomic.Int32
	s := cron.New(cron.WithJitter(0))
	s.Every(10*time.Millisecond, "tick", func(context.Context) error {
		calls.Add(1)
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	s.Start(ctx)
	<-ctx.Done()
	s.Stop()

	assert.GreaterOrEqual(t, calls.Load(), int32(3))
}

func TestScheduler_StopWaitsForInflightJob(t *testing.T) {
	var (
		started  atomic.Bool
		returned atomic.Bool
	)
	s := cron.New(cron.WithJitter(0))
	s.Every(5*time.Millisecond, "slow", func(ctx context.Context) error {
		started.Store(true)
		select {
		case <-time.After(40 * time.Millisecond):
		case <-ctx.Done():
		}
		returned.Store(true)
		return nil
	})

	s.Start(context.Background())
	for i := 0; i < 50 && !started.Load(); i++ {
		time.Sleep(2 * time.Millisecond)
	}
	require.True(t, started.Load(), "job must start before we call Stop")

	s.Stop()
	assert.True(t, returned.Load(), "Stop must block until the inflight job returns")
}

func TestScheduler_RecoversFromPanic(t *testing.T) {
	var calls atomic.Int32
	s := cron.New(cron.WithJitter(0))
	s.Every(5*time.Millisecond, "panicky", func(context.Context) error {
		calls.Add(1)
		panic("boom")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	s.Start(ctx)
	<-ctx.Done()
	s.Stop()

	assert.Greater(t, calls.Load(), int32(2), "scheduler must keep firing despite panics")
	stats := s.Stats()
	require.Len(t, stats, 1)
	assert.Contains(t, stats[0].LastError, "panic")
}

func TestScheduler_StatsTrackFailures(t *testing.T) {
	s := cron.New(cron.WithJitter(0))
	s.Every(5*time.Millisecond, "flaky", func(context.Context) error {
		return errors.New("oops")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	s.Start(ctx)
	<-ctx.Done()
	s.Stop()

	stats := s.Stats()
	require.Len(t, stats, 1)
	assert.Equal(t, "flaky", stats[0].Name)
	assert.Greater(t, stats[0].Runs, int64(0))
	assert.Equal(t, stats[0].Runs, stats[0].Failures)
}

func TestScheduler_RespectsJobTimeout(t *testing.T) {
	var hit atomic.Bool
	s := cron.New(cron.WithJitter(0), cron.WithJobTimeout(10*time.Millisecond))
	s.Every(20*time.Millisecond, "slow", func(ctx context.Context) error {
		select {
		case <-time.After(100 * time.Millisecond):
			return nil
		case <-ctx.Done():
			hit.Store(true)
			return ctx.Err()
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	s.Start(ctx)
	<-ctx.Done()
	s.Stop()

	assert.True(t, hit.Load(), "job must observe its own timeout")
}

func TestScheduler_EveryPanicsAfterStart(t *testing.T) {
	s := cron.New()
	s.Start(context.Background())
	defer s.Stop()

	assert.Panics(t, func() {
		s.Every(time.Second, "late", func(context.Context) error { return nil })
	})
}

func TestScheduler_EveryRejectsZeroInterval(t *testing.T) {
	s := cron.New()
	assert.Panics(t, func() {
		s.Every(0, "zero", func(context.Context) error { return nil })
	})
}

func TestScheduler_StopBeforeStartIsSafe(t *testing.T) {
	s := cron.New()
	assert.NotPanics(t, func() {
		s.Stop()
	})
}
