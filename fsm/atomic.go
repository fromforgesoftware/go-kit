package fsm

import (
	"context"
	"sync/atomic"
)

// AtomicState is the state type used by an AtomicMachine. Backed by a
// uint32 so it fits atomic.Uint32; consumers cast their own state enums.
type AtomicState uint32

// AtomicEvent is the event type used by an AtomicMachine. Same uint32
// backing for the same reason.
type AtomicEvent uint32

// AtomicTransition is a transition declared by index rather than by
// generic comparable; the trade-off vs Machine[S,E] is no compile-time
// type safety on the state/event symbols, traded for zero-allocation
// atomic-only transitions.
type AtomicTransition struct {
	From  AtomicState
	Event AtomicEvent
	To    AtomicState
	Guard func(ctx context.Context) bool
}

// AtomicSpec describes an AtomicMachine. Validation is the caller's
// responsibility — this type is for the hot path, where Machine[S,E]'s
// safety checks would impose unnecessary cost.
type AtomicSpec struct {
	Initial     AtomicState
	Transitions []AtomicTransition
}

// AtomicMachine is a lock-free single-writer-friendly FSM. State is held
// in an atomic.Uint32; State() and IsState() are concurrent-safe lookups.
// Send must be called from a single goroutine — concurrent Send may lose
// transitions under a CAS race.
//
// Use this when the consumer is an actor (one goroutine owns the
// machine) and many readers need to query state without contention.
// For general-purpose use prefer Machine[S, E].
type AtomicMachine struct {
	spec  AtomicSpec
	state atomic.Uint32
}

// NewAtomic constructs an AtomicMachine.
func NewAtomic(spec AtomicSpec) *AtomicMachine {
	m := &AtomicMachine{spec: spec}
	m.state.Store(uint32(spec.Initial))
	return m
}

// State returns the current state. Safe to call from any goroutine.
func (m *AtomicMachine) State() AtomicState {
	return AtomicState(m.state.Load())
}

// IsState reports whether the machine is in s. Cheaper than comparing
// State() when you only need a boolean.
func (m *AtomicMachine) IsState(s AtomicState) bool {
	return m.state.Load() == uint32(s)
}

// Send delivers an event. Caller-side single-writer: concurrent Send
// from multiple goroutines is not safe.
func (m *AtomicMachine) Send(ctx context.Context, ev AtomicEvent) error {
	cur := AtomicState(m.state.Load())
	for _, t := range m.spec.Transitions {
		if t.From != cur || t.Event != ev {
			continue
		}
		if t.Guard != nil && !t.Guard(ctx) {
			continue
		}
		m.state.Store(uint32(t.To))
		return nil
	}
	return ErrIllegalTransition
}
