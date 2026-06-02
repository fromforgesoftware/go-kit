package fsm_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/fsm"
)

type state string
type event string

const (
	closed state = "closed"
	open   state = "open"
	locked state = "locked"

	evOpen   event = "open"
	evClose  event = "close"
	evLock   event = "lock"
	evUnlock event = "unlock"
)

func doorSpec() fsm.Spec[state, event] {
	return fsm.Spec[state, event]{
		Initial: closed,
		Transitions: []fsm.Transition[state, event]{
			{From: closed, Event: evOpen, To: open},
			{From: open, Event: evClose, To: closed},
			{From: closed, Event: evLock, To: locked},
			{From: locked, Event: evUnlock, To: closed},
		},
	}
}

func TestBasicTransitions(t *testing.T) {
	m := fsm.New(doorSpec())
	assert.Equal(t, closed, m.State())

	require.NoError(t, m.Send(context.Background(), evOpen))
	assert.Equal(t, open, m.State())

	require.NoError(t, m.Send(context.Background(), evClose))
	assert.Equal(t, closed, m.State())
}

func TestIllegalTransition(t *testing.T) {
	m := fsm.New(doorSpec())
	err := m.Send(context.Background(), evUnlock)
	assert.ErrorIs(t, err, fsm.ErrIllegalTransition)
}

func TestOnEnterOnExitHooks(t *testing.T) {
	enters := []state{}
	exits := []state{}
	spec := doorSpec()
	spec.OnEnter = map[state]func(context.Context){
		open: func(ctx context.Context) { enters = append(enters, open) },
	}
	spec.OnExit = map[state]func(context.Context){
		closed: func(ctx context.Context) { exits = append(exits, closed) },
	}
	m := fsm.New(spec)
	require.NoError(t, m.Send(context.Background(), evOpen))
	assert.Equal(t, []state{open}, enters)
	assert.Equal(t, []state{closed}, exits)
}

func TestGuardBlocks(t *testing.T) {
	spec := fsm.Spec[state, event]{
		Initial: closed,
		Transitions: []fsm.Transition[state, event]{
			{From: closed, Event: evOpen, To: open, Guard: func(ctx context.Context) bool { return false }},
		},
	}
	m := fsm.New(spec)
	assert.ErrorIs(t, m.Send(context.Background(), evOpen), fsm.ErrIllegalTransition)
}

func TestHistory(t *testing.T) {
	spec := doorSpec()
	spec.HistorySize = 4
	m := fsm.New(spec)
	require.NoError(t, m.Send(context.Background(), evOpen))
	require.NoError(t, m.Send(context.Background(), evClose))
	require.NoError(t, m.Send(context.Background(), evLock))
	assert.Equal(t, []state{closed, open, closed, locked}, m.History())
}

func TestConcurrentSend(t *testing.T) {
	m := fsm.New(doorSpec())
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.Send(context.Background(), evOpen)
			_ = m.Send(context.Background(), evClose)
		}()
	}
	wg.Wait()
	// final state is well-defined: either closed or open, never garbage
	s := m.State()
	assert.Contains(t, []state{closed, open}, s)
}

func TestValidateRejectsDuplicateGuardless(t *testing.T) {
	spec := fsm.Spec[state, event]{
		Initial: closed,
		Transitions: []fsm.Transition[state, event]{
			{From: closed, Event: evOpen, To: open},
			{From: closed, Event: evOpen, To: locked},
		},
	}
	assert.ErrorIs(t, fsm.Validate(spec), fsm.ErrInvalidSpec)
}

func TestValidateRejectsEmptyTransitions(t *testing.T) {
	assert.ErrorIs(t, fsm.Validate(fsm.Spec[state, event]{Initial: closed}), fsm.ErrInvalidSpec)
}
