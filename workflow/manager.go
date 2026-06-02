package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// WorkflowID uniquely identifies a workflow instance.
type WorkflowID string

// Idempotency is a client-supplied key that makes Send safe to retry.
type Idempotency string

// Transition captures a single state change applied to an instance.
type Transition struct {
	From string
	To   string
	Ev   string
	At   time.Time
}

// Spec defines the workflow shape. State and Event are constrained to
// ~string so they can be persisted as-is.
type Spec[S ~string, E ~string] struct {
	Initial     S
	Transitions []Edge[S, E]
	OnEnter     map[S]func(ctx context.Context, id WorkflowID) error
	OnExit      map[S]func(ctx context.Context, id WorkflowID) error
	// Compensators run during Compensate(); each Compensator unwinds the
	// effect of *entering* its state.
	Compensators map[S]func(ctx context.Context, id WorkflowID) error
}

// Edge is one transition rule.
type Edge[S ~string, E ~string] struct {
	From  S
	Event E
	To    S
	Guard func(ctx context.Context, id WorkflowID) bool
}

// Errors surfaced by the manager.
var (
	ErrUnknownInstance   = errors.New("workflow: unknown instance")
	ErrIllegalTransition = errors.New("workflow: illegal transition")
	ErrAlreadyStarted    = errors.New("workflow: instance already started")
)

// Manager drives workflow instances against a backing Store.
type Manager[S ~string, E ~string] struct {
	spec  Spec[S, E]
	store Store
	clock func() time.Time

	mu   sync.Mutex
	seen map[Idempotency]struct{}
}

// NewManager constructs a Manager from a Spec and Store.
func NewManager[S ~string, E ~string](spec Spec[S, E], store Store) *Manager[S, E] {
	return &Manager[S, E]{
		spec:  spec,
		store: store,
		clock: time.Now,
		seen:  map[Idempotency]struct{}{},
	}
}

// SetClock overrides the wall clock; useful for tests.
func (m *Manager[S, E]) SetClock(f func() time.Time) { m.clock = f }

// Start begins a new workflow instance at the spec's initial state.
func (m *Manager[S, E]) Start(ctx context.Context, id WorkflowID) error {
	if _, err := m.store.CurrentState(ctx, id); err == nil {
		return ErrAlreadyStarted
	}
	now := m.clock()
	t := Transition{From: "", To: string(m.spec.Initial), Ev: "__start__", At: now}
	if err := m.store.SaveTransition(ctx, id, t); err != nil {
		return err
	}
	if onEnter := m.spec.OnEnter[m.spec.Initial]; onEnter != nil {
		if err := onEnter(ctx, id); err != nil {
			return fmt.Errorf("workflow: OnEnter %s: %w", m.spec.Initial, err)
		}
	}
	return nil
}

// Send delivers an event to the workflow. The transition (if any) is
// persisted before OnExit/OnEnter run, so a crash mid-callback is
// recoverable via Resume.
func (m *Manager[S, E]) Send(ctx context.Context, id WorkflowID, ev E, key Idempotency) error {
	m.mu.Lock()
	if key != "" {
		if _, ok := m.seen[key]; ok {
			m.mu.Unlock()
			return nil
		}
		m.seen[key] = struct{}{}
	}
	m.mu.Unlock()

	cur, err := m.store.CurrentState(ctx, id)
	if err != nil {
		return ErrUnknownInstance
	}
	from := S(cur)
	for _, t := range m.spec.Transitions {
		if t.From != from || t.Event != ev {
			continue
		}
		if t.Guard != nil && !t.Guard(ctx, id) {
			continue
		}
		now := m.clock()
		tr := Transition{From: string(from), To: string(t.To), Ev: string(ev), At: now}
		if err := m.store.SaveTransition(ctx, id, tr); err != nil {
			return err
		}
		if onExit := m.spec.OnExit[from]; onExit != nil {
			if err := onExit(ctx, id); err != nil {
				return fmt.Errorf("workflow: OnExit %s: %w", from, err)
			}
		}
		if onEnter := m.spec.OnEnter[t.To]; onEnter != nil {
			if err := onEnter(ctx, id); err != nil {
				return fmt.Errorf("workflow: OnEnter %s: %w", t.To, err)
			}
		}
		return nil
	}
	return ErrIllegalTransition
}

// State returns the current state of an instance.
func (m *Manager[S, E]) State(ctx context.Context, id WorkflowID) (S, error) {
	s, err := m.store.CurrentState(ctx, id)
	if err != nil {
		return "", ErrUnknownInstance
	}
	return S(s), nil
}

// Compensate walks history backwards from the current state until
// `toState` (inclusive of leaving the current state, exclusive of
// entering `toState` itself) and invokes each Compensator. Useful when
// a downstream failure forces the workflow to undo recent transitions.
func (m *Manager[S, E]) Compensate(ctx context.Context, id WorkflowID, toState S) error {
	hist, err := m.store.History(ctx, id)
	if err != nil {
		return err
	}
	for i := len(hist) - 1; i >= 0; i-- {
		st := S(hist[i].To)
		if st == toState {
			break
		}
		if comp := m.spec.Compensators[st]; comp != nil {
			if err := comp(ctx, id); err != nil {
				return fmt.Errorf("workflow: compensate %s: %w", st, err)
			}
		}
		// Record the rollback as a transition for auditability.
		now := m.clock()
		rev := Transition{From: hist[i].To, To: hist[i].From, Ev: "__compensate__", At: now}
		if err := m.store.SaveTransition(ctx, id, rev); err != nil {
			return err
		}
	}
	return nil
}

// Resume is a hook for process startup to do any housekeeping on
// in-flight instances (the Store may need to load state into memory).
// The default implementation is a no-op — workflow state lives in the
// store and is read on-demand. Callers can override per-instance
// recovery by extending the Store.
func (m *Manager[S, E]) Resume(_ context.Context) error { return nil }
