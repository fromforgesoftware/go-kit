package timeline

import (
	"sort"
	"time"
)

// Event is a typed payload at a fixed offset from the timeline start.
type Event[E any] struct {
	Offset  time.Duration
	Payload E
}

// Timeline is an immutable, sorted sequence of Events.
type Timeline[E any] struct {
	events []Event[E]
}

// New constructs a Timeline. Events are sorted by Offset; the input slice
// is not mutated.
func New[E any](events []Event[E]) *Timeline[E] {
	sorted := make([]Event[E], len(events))
	copy(sorted, events)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Offset < sorted[j].Offset
	})
	return &Timeline[E]{events: sorted}
}

// Events returns a read-only snapshot of the timeline's events.
func (t *Timeline[E]) Events() []Event[E] {
	out := make([]Event[E], len(t.events))
	copy(out, t.events)
	return out
}

// Duration returns the offset of the last event (zero if empty).
func (t *Timeline[E]) Duration() time.Duration {
	if len(t.events) == 0 {
		return 0
	}
	return t.events[len(t.events)-1].Offset
}

// Merge combines two timelines into one preserving global order.
func Merge[E any](tls ...*Timeline[E]) *Timeline[E] {
	all := []Event[E]{}
	for _, t := range tls {
		all = append(all, t.events...)
	}
	return New(all)
}
