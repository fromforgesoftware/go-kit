package idgen

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withClock replaces the generator's clock with one driven by the returned
// setter, so tests can simulate the wall clock moving backward.
func withClock(g *Generator, initial int64) func(int64) {
	var (
		mu  sync.Mutex
		cur = initial
	)
	g.nowFn = func() int64 {
		mu.Lock()
		defer mu.Unlock()
		return cur
	}
	return func(v int64) {
		mu.Lock()
		cur = v
		mu.Unlock()
	}
}

func TestNextIDErr_ClockBackward(t *testing.T) {
	g := NewGenerator(1)
	set := withClock(g, 1_000_000)

	// First call advances lastTime to the initial clock value.
	_, err := g.NextIDErr()
	require.NoError(t, err)

	// Move the clock backward; the generator must refuse rather than panic.
	set(999_999)
	id, err := g.NextIDErr()
	require.Error(t, err)
	assert.Zero(t, id)
	assert.ErrorIs(t, err, ErrClockBackward)
}

func TestNextIDErr_RecoversWhenClockCatchesUp(t *testing.T) {
	g := NewGenerator(1)
	set := withClock(g, 2_000_000)

	_, err := g.NextIDErr()
	require.NoError(t, err)

	// Clock goes backward: error.
	set(1_999_000)
	_, err = g.NextIDErr()
	require.ErrorIs(t, err, ErrClockBackward)

	// Clock catches up past the last observed time: generation resumes.
	set(2_000_001)
	id, err := g.NextIDErr()
	require.NoError(t, err)
	assert.NotZero(t, id)
}

// TestNextID_PanicsOnClockBackward documents that the deprecated NextID still
// panics on a backward clock, preserving its historical behaviour.
func TestNextID_PanicsOnClockBackward(t *testing.T) {
	g := NewGenerator(1)
	set := withClock(g, 5_000_000)

	require.NotPanics(t, func() { g.NextID() })

	set(4_000_000)
	defer func() {
		r := recover()
		require.NotNil(t, r, "NextID should panic when the clock moves backward")
		msg, ok := r.(string)
		require.True(t, ok, "panic value should be a string, got %T", r)
		assert.True(t, strings.Contains(msg, "clock moved backward"),
			"panic message %q should mention the clock moving backward", msg)
	}()
	_ = g.NextID()
}
