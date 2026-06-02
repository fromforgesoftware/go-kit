package workflow

import (
	"context"
	"errors"
	"sync"
)

// Store persists workflow transitions and exposes per-instance current
// state and history.
type Store interface {
	SaveTransition(ctx context.Context, id WorkflowID, t Transition) error
	CurrentState(ctx context.Context, id WorkflowID) (string, error)
	History(ctx context.Context, id WorkflowID) ([]Transition, error)
}

// ErrInstanceMissing is returned when CurrentState is called for an
// instance that was never started.
var ErrInstanceMissing = errors.New("workflow: instance not found")

// MemoryStore is an in-process Store. Goroutine-safe.
type MemoryStore struct {
	mu      sync.Mutex
	history map[WorkflowID][]Transition
}

// NewMemoryStore constructs an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{history: map[WorkflowID][]Transition{}}
}

// SaveTransition implements Store.
func (s *MemoryStore) SaveTransition(_ context.Context, id WorkflowID, t Transition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history[id] = append(s.history[id], t)
	return nil
}

// CurrentState implements Store.
func (s *MemoryStore) CurrentState(_ context.Context, id WorkflowID) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.history[id]
	if !ok || len(h) == 0 {
		return "", ErrInstanceMissing
	}
	return h[len(h)-1].To, nil
}

// History implements Store.
func (s *MemoryStore) History(_ context.Context, id WorkflowID) ([]Transition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.history[id]
	out := make([]Transition, len(h))
	copy(out, h)
	return out, nil
}
