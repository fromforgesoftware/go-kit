package timeline_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/go-kit/timeline"
)

type note string

func TestNewSortsEvents(t *testing.T) {
	tl := timeline.New([]timeline.Event[note]{
		{Offset: 200 * time.Millisecond, Payload: "b"},
		{Offset: 50 * time.Millisecond, Payload: "a"},
		{Offset: 300 * time.Millisecond, Payload: "c"},
	})
	events := tl.Events()
	assert.Equal(t, note("a"), events[0].Payload)
	assert.Equal(t, note("b"), events[1].Payload)
	assert.Equal(t, note("c"), events[2].Payload)
}

func TestCursorAdvanceEmitsInRange(t *testing.T) {
	tl := timeline.New([]timeline.Event[note]{
		{Offset: 100 * time.Millisecond, Payload: "a"},
		{Offset: 200 * time.Millisecond, Payload: "b"},
		{Offset: 300 * time.Millisecond, Payload: "c"},
	})
	p := timeline.NewCursor(tl)
	var emitted []note

	p.Advance(150*time.Millisecond, func(ev timeline.Event[note]) { emitted = append(emitted, ev.Payload) })
	assert.Equal(t, []note{"a"}, emitted)

	emitted = nil
	p.Advance(100*time.Millisecond, func(ev timeline.Event[note]) { emitted = append(emitted, ev.Payload) })
	assert.Equal(t, []note{"b"}, emitted)

	emitted = nil
	p.Advance(100*time.Millisecond, func(ev timeline.Event[note]) { emitted = append(emitted, ev.Payload) })
	assert.Equal(t, []note{"c"}, emitted)
}

func TestCursorSeekBackwardReplays(t *testing.T) {
	tl := timeline.New([]timeline.Event[note]{
		{Offset: 100 * time.Millisecond, Payload: "a"},
		{Offset: 200 * time.Millisecond, Payload: "b"},
	})
	p := timeline.NewCursor(tl)
	p.Advance(300*time.Millisecond, func(_ timeline.Event[note]) {})

	var replayed []note
	p.Seek(150*time.Millisecond, func(ev timeline.Event[note]) { replayed = append(replayed, ev.Payload) })
	assert.Equal(t, []note{"a"}, replayed)
}

func TestCursorDeterminism(t *testing.T) {
	tl := timeline.New([]timeline.Event[note]{
		{Offset: 50 * time.Millisecond, Payload: "x"},
		{Offset: 75 * time.Millisecond, Payload: "y"},
		{Offset: 90 * time.Millisecond, Payload: "z"},
	})

	collect := func(steps []time.Duration) []note {
		p := timeline.NewCursor(tl)
		var got []note
		for _, s := range steps {
			p.Advance(s, func(ev timeline.Event[note]) { got = append(got, ev.Payload) })
		}
		return got
	}

	a := collect([]time.Duration{30 * time.Millisecond, 50 * time.Millisecond, 20 * time.Millisecond})
	b := collect([]time.Duration{10 * time.Millisecond, 90 * time.Millisecond})
	assert.Equal(t, a, b, "same total advance should produce same emission sequence")
}

func TestMerge(t *testing.T) {
	a := timeline.New([]timeline.Event[note]{{Offset: 100 * time.Millisecond, Payload: "a"}})
	b := timeline.New([]timeline.Event[note]{{Offset: 50 * time.Millisecond, Payload: "b"}})
	merged := timeline.Merge(a, b)
	events := merged.Events()
	assert.Equal(t, note("b"), events[0].Payload)
	assert.Equal(t, note("a"), events[1].Payload)
}
