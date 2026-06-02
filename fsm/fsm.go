package fsm

import (
	"context"
	"errors"
	"sync"
)

// ErrIllegalTransition is returned when no transition matches the current
// (state, event) pair (after guard evaluation).
var ErrIllegalTransition = errors.New("fsm: illegal transition")

// Transition declares a movement from From → To when Event arrives. Guard,
// if non-nil, must return true for the transition to fire.
type Transition[S, E comparable] struct {
	From  S
	Event E
	To    S
	Guard func(ctx context.Context) bool
}

// Spec is the declarative definition of a machine. Pass by value to New.
type Spec[S, E comparable] struct {
	Initial     S
	Transitions []Transition[S, E]
	OnEnter     map[S]func(ctx context.Context)
	OnExit      map[S]func(ctx context.Context)
	// HistorySize bounds the recorded state history (0 = disabled).
	HistorySize int
}

// Machine is the live FSM. Safe for concurrent use.
type Machine[S, E comparable] struct {
	spec    Spec[S, E]
	state   S
	history []S
	mu      sync.Mutex
}

// New constructs a Machine from a Spec. Validates the spec; nil is never
// returned (panics on invalid spec).
func New[S, E comparable](spec Spec[S, E]) *Machine[S, E] {
	if err := Validate(spec); err != nil {
		panic(err)
	}
	m := &Machine[S, E]{spec: spec, state: spec.Initial}
	if spec.HistorySize > 0 {
		m.history = make([]S, 0, spec.HistorySize)
		m.history = append(m.history, spec.Initial)
	}
	return m
}

// State returns the current state.
func (m *Machine[S, E]) State() S {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// Send delivers an event. Returns ErrIllegalTransition if no transition
// from the current state matches the event (or all matching guards fail).
func (m *Machine[S, E]) Send(ctx context.Context, ev E) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.spec.Transitions {
		if t.From != m.state || t.Event != ev {
			continue
		}
		if t.Guard != nil && !t.Guard(ctx) {
			continue
		}
		if onExit := m.spec.OnExit[m.state]; onExit != nil {
			onExit(ctx)
		}
		m.state = t.To
		if onEnter := m.spec.OnEnter[t.To]; onEnter != nil {
			onEnter(ctx)
		}
		if m.spec.HistorySize > 0 {
			if len(m.history) == m.spec.HistorySize {
				copy(m.history, m.history[1:])
				m.history = m.history[:len(m.history)-1]
			}
			m.history = append(m.history, t.To)
		}
		return nil
	}
	return ErrIllegalTransition
}

// History returns a copy of the recorded state history (oldest first).
// Empty unless Spec.HistorySize > 0.
func (m *Machine[S, E]) History() []S {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]S, len(m.history))
	copy(out, m.history)
	return out
}
