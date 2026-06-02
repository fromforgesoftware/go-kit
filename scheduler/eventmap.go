package scheduler

import "time"

// EventID is an opaque identifier supplied by the caller.
type EventID uint32

// GroupID groups events such that only one in a group is active at a time
// (new schedules into a group cancel any existing). Use 0 for ungrouped.
type GroupID uint32

// EventMap is a min-heap-backed event queue keyed by due time. Schedule
// and Update are O(log n); Update emits due events in chronological order.
type EventMap struct {
	clock Clock
	heap  eventHeap
}

type eventEntry struct {
	id    EventID
	group GroupID
	due   time.Time
}

// NewEventMap constructs an EventMap. Pass SystemClock{} for real time.
func NewEventMap(clock Clock) *EventMap {
	return &EventMap{clock: clock}
}

// Schedule inserts an event due `delay` after the current clock time. If
// the group is non-zero, any existing event in the same group is cancelled.
func (m *EventMap) Schedule(id EventID, delay time.Duration, group GroupID) {
	if group != 0 {
		m.cancelGroup(group)
	}
	due := m.clock.Now().Add(delay)
	m.heap.push(eventEntry{id: id, group: group, due: due})
}

func (m *EventMap) cancelGroup(group GroupID) {
	// Lazy: filter by rebuilding the slice. Group cancellations are rare
	// relative to Schedule/Update so a per-group index would be premature.
	out := m.heap.items[:0]
	for _, e := range m.heap.items {
		if e.group != group {
			out = append(out, e)
		}
	}
	m.heap.items = out
	m.heap.heapify()
}

// Reschedule moves an existing event to a new due time. No-op if id not found.
func (m *EventMap) Reschedule(id EventID, delay time.Duration) {
	due := m.clock.Now().Add(delay)
	for i := range m.heap.items {
		if m.heap.items[i].id == id {
			m.heap.items[i].due = due
			m.heap.heapify()
			return
		}
	}
}

// Cancel removes an event. No-op if not present.
func (m *EventMap) Cancel(id EventID) {
	for i := range m.heap.items {
		if m.heap.items[i].id == id {
			last := len(m.heap.items) - 1
			m.heap.items[i] = m.heap.items[last]
			m.heap.items = m.heap.items[:last]
			m.heap.heapify()
			return
		}
	}
}

// Update drains events whose due time is at or before `now` into `due`,
// preserving the caller's slice for allocation-free reuse. Events are
// emitted in chronological order.
func (m *EventMap) Update(now time.Time, due *[]EventID) {
	*due = (*due)[:0]
	for m.heap.Len() > 0 {
		top := m.heap.peek()
		if now.Before(top.due) {
			return
		}
		*due = append(*due, m.heap.pop().id)
	}
}

// eventHeap is a min-heap over eventEntry.due, hand-rolled to avoid the
// interface{} boxing container/heap forces.
type eventHeap struct {
	items []eventEntry
}

func (h *eventHeap) Len() int { return len(h.items) }

func (h *eventHeap) peek() eventEntry { return h.items[0] }

func (h *eventHeap) push(e eventEntry) {
	h.items = append(h.items, e)
	h.siftUp(len(h.items) - 1)
}

func (h *eventHeap) pop() eventEntry {
	top := h.items[0]
	last := len(h.items) - 1
	h.items[0] = h.items[last]
	h.items = h.items[:last]
	if len(h.items) > 0 {
		h.siftDown(0)
	}
	return top
}

func (h *eventHeap) siftUp(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if !h.items[i].due.Before(h.items[parent].due) {
			return
		}
		h.items[i], h.items[parent] = h.items[parent], h.items[i]
		i = parent
	}
}

func (h *eventHeap) siftDown(i int) {
	n := len(h.items)
	for {
		left := 2*i + 1
		right := 2*i + 2
		smallest := i
		if left < n && h.items[left].due.Before(h.items[smallest].due) {
			smallest = left
		}
		if right < n && h.items[right].due.Before(h.items[smallest].due) {
			smallest = right
		}
		if smallest == i {
			return
		}
		h.items[i], h.items[smallest] = h.items[smallest], h.items[i]
		i = smallest
	}
}

// heapify restores the heap invariant on the full items slice. Used after
// mid-heap mutations (Cancel, cancelGroup, Reschedule).
func (h *eventHeap) heapify() {
	for i := len(h.items)/2 - 1; i >= 0; i-- {
		h.siftDown(i)
	}
}
