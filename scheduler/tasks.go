package scheduler

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// TaskHandle identifies a scheduled task. Use it to Cancel before fire.
type TaskHandle uint64

// TaskScheduler is a callback-based scheduler supporting one-shot and
// repeating tasks with conditional predicates.
type TaskScheduler struct {
	clock   Clock
	mu      sync.Mutex
	tasks   map[TaskHandle]*task
	nextID  uint64
	pending []scheduledTask
}

type task struct {
	fn        func(ctx context.Context)
	repeats   int // -1 = forever
	interval  time.Duration
	predicate func() bool
	cancelled atomic.Bool
}

type scheduledTask struct {
	handle TaskHandle
	due    time.Time
}

// NewTaskScheduler returns a TaskScheduler. Update() must be called from
// a single goroutine — usually the tick loop.
func NewTaskScheduler(clock Clock) *TaskScheduler {
	return &TaskScheduler{clock: clock, tasks: make(map[TaskHandle]*task)}
}

// TaskOpt customises a scheduled task.
type TaskOpt func(*task)

// WithRepeats fires the task n additional times at the same interval after
// the initial fire. -1 = forever.
func WithRepeats(n int) TaskOpt {
	return func(t *task) { t.repeats = n }
}

// WithPredicate makes the task fire only when pred() returns true. Returning
// false skips this firing without consuming a repeat.
func WithPredicate(pred func() bool) TaskOpt {
	return func(t *task) { t.predicate = pred }
}

// Schedule adds a task due after `delay`. Returns a handle for cancellation.
func (s *TaskScheduler) Schedule(delay time.Duration, fn func(ctx context.Context), opts ...TaskOpt) TaskHandle {
	t := &task{fn: fn, interval: delay}
	for _, o := range opts {
		o(t)
	}
	h := TaskHandle(atomic.AddUint64(&s.nextID, 1))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[h] = t
	s.pending = append(s.pending, scheduledTask{handle: h, due: s.clock.Now().Add(delay)})
	return h
}

// Cancel marks a task cancelled; the next Update drop pass removes it.
func (s *TaskScheduler) Cancel(h TaskHandle) {
	s.mu.Lock()
	t, ok := s.tasks[h]
	s.mu.Unlock()
	if !ok {
		return
	}
	t.cancelled.Store(true)
}

// Update fires all due tasks at or before `now`. Must be called from a
// single goroutine.
func (s *TaskScheduler) Update(ctx context.Context, now time.Time) {
	s.mu.Lock()
	sort.SliceStable(s.pending, func(i, j int) bool {
		return s.pending[i].due.Before(s.pending[j].due)
	})
	keep := s.pending[:0]
	fire := []scheduledTask{}
	for _, p := range s.pending {
		t, ok := s.tasks[p.handle]
		if !ok || t.cancelled.Load() {
			delete(s.tasks, p.handle)
			continue
		}
		if !now.Before(p.due) {
			fire = append(fire, p)
		} else {
			keep = append(keep, p)
		}
	}
	s.pending = keep
	s.mu.Unlock()

	for _, p := range fire {
		s.mu.Lock()
		t, ok := s.tasks[p.handle]
		s.mu.Unlock()
		if !ok || t.cancelled.Load() {
			continue
		}
		shouldRun := t.predicate == nil || t.predicate()
		if shouldRun {
			t.fn(ctx)
		}
		// Re-schedule if repeats remain.
		if t.repeats == -1 || t.repeats > 0 {
			if t.repeats > 0 {
				t.repeats--
			}
			s.mu.Lock()
			s.pending = append(s.pending, scheduledTask{handle: p.handle, due: now.Add(t.interval)})
			s.mu.Unlock()
		} else {
			s.mu.Lock()
			delete(s.tasks, p.handle)
			s.mu.Unlock()
		}
	}
}
