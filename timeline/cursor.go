package timeline

import "time"

// Cursor advances through a Timeline, emitting events whose offset falls
// within the most recently advanced span. The same shape as a media
// playback head — domain-agnostic.
type Cursor[E any] struct {
	tl  *Timeline[E]
	pos time.Duration
	idx int
}

// NewCursor returns a Cursor starting at offset 0.
func NewCursor[E any](tl *Timeline[E]) *Cursor[E] {
	return &Cursor[E]{tl: tl}
}

// Reset returns the cursor to offset 0.
func (c *Cursor[E]) Reset() {
	c.pos = 0
	c.idx = 0
}

// Offset returns the current cursor position along the timeline.
func (c *Cursor[E]) Offset() time.Duration { return c.pos }

// Advance moves the cursor by `by` and invokes fn for every event in
// (oldOffset, newOffset]. Events at the same offset emit in insertion
// order (stable sort).
func (c *Cursor[E]) Advance(by time.Duration, fn func(ev Event[E])) {
	if by <= 0 {
		return
	}
	newPos := c.pos + by
	for c.idx < len(c.tl.events) && c.tl.events[c.idx].Offset <= newPos {
		fn(c.tl.events[c.idx])
		c.idx++
	}
	c.pos = newPos
}

// Seek moves the cursor to `to`, emitting any events crossed since the
// previous position. Seeking backwards Resets and replays forward.
func (c *Cursor[E]) Seek(to time.Duration, fn func(ev Event[E])) {
	if to < c.pos {
		c.Reset()
	}
	c.Advance(to-c.pos, fn)
}
