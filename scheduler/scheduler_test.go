package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/scheduler"
)

func TestEventMapFiresInOrder(t *testing.T) {
	clk := scheduler.NewMockClock(time.Unix(0, 0))
	m := scheduler.NewEventMap(clk)
	m.Schedule(2, 200*time.Millisecond, 0)
	m.Schedule(1, 100*time.Millisecond, 0)

	var out []scheduler.EventID

	clk.Add(150 * time.Millisecond)
	m.Update(clk.Now(), &out)
	assert.Equal(t, []scheduler.EventID{1}, out)

	clk.Add(100 * time.Millisecond)
	m.Update(clk.Now(), &out)
	assert.Equal(t, []scheduler.EventID{2}, out)
}

func TestEventMapGroupCancelsExisting(t *testing.T) {
	clk := scheduler.NewMockClock(time.Unix(0, 0))
	m := scheduler.NewEventMap(clk)
	m.Schedule(1, 100*time.Millisecond, 7)
	m.Schedule(2, 200*time.Millisecond, 7)

	var out []scheduler.EventID
	clk.Add(500 * time.Millisecond)
	m.Update(clk.Now(), &out)
	assert.Equal(t, []scheduler.EventID{2}, out, "second schedule in same group cancels the first")
}

func TestTaskSchedulerFires(t *testing.T) {
	clk := scheduler.NewMockClock(time.Unix(0, 0))
	s := scheduler.NewTaskScheduler(clk)
	fired := 0
	s.Schedule(100*time.Millisecond, func(_ context.Context) { fired++ })

	clk.Add(50 * time.Millisecond)
	s.Update(context.Background(), clk.Now())
	assert.Equal(t, 0, fired)

	clk.Add(50 * time.Millisecond)
	s.Update(context.Background(), clk.Now())
	assert.Equal(t, 1, fired)
}

func TestTaskSchedulerRepeats(t *testing.T) {
	clk := scheduler.NewMockClock(time.Unix(0, 0))
	s := scheduler.NewTaskScheduler(clk)
	fired := 0
	s.Schedule(100*time.Millisecond, func(_ context.Context) { fired++ }, scheduler.WithRepeats(2))

	for i := 0; i < 5; i++ {
		clk.Add(100 * time.Millisecond)
		s.Update(context.Background(), clk.Now())
	}
	assert.Equal(t, 3, fired, "initial + 2 repeats = 3")
}

func TestTaskSchedulerPredicate(t *testing.T) {
	clk := scheduler.NewMockClock(time.Unix(0, 0))
	s := scheduler.NewTaskScheduler(clk)
	fired := 0
	allow := false
	s.Schedule(100*time.Millisecond, func(_ context.Context) { fired++ },
		scheduler.WithRepeats(3),
		scheduler.WithPredicate(func() bool { return allow }))

	clk.Add(100 * time.Millisecond)
	s.Update(context.Background(), clk.Now())
	require.Equal(t, 0, fired)

	allow = true
	clk.Add(100 * time.Millisecond)
	s.Update(context.Background(), clk.Now())
	assert.Equal(t, 1, fired)
}

func TestBucketsTickRespectsInterval(t *testing.T) {
	type unit struct {
		Hot bool
	}
	b := scheduler.NewBuckets([]scheduler.Bucket{
		{Name: "hot", Interval: 100 * time.Millisecond},
		{Name: "cold", Interval: 1000 * time.Millisecond},
	}, func(u unit) string {
		if u.Hot {
			return "hot"
		}
		return "cold"
	})
	b.Add(1, unit{Hot: true})
	b.Add(2, unit{Hot: false})

	hotTicks, coldTicks := 0, 0
	start := time.Unix(0, 0)
	for offset := time.Duration(0); offset < time.Second; offset += 100 * time.Millisecond {
		b.Tick(start.Add(offset), func(u unit) {
			if u.Hot {
				hotTicks++
			} else {
				coldTicks++
			}
		})
	}
	assert.Equal(t, 10, hotTicks)
	assert.Equal(t, 1, coldTicks)
}
