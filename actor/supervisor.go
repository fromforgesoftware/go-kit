package actor

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RestartPolicy controls how a Supervisor responds to a child panic/error.
type RestartPolicy struct {
	// Backoff is the duration to wait before restarting.
	Backoff time.Duration
	// MaxRestarts caps total restarts; 0 = unlimited.
	MaxRestarts int
}

// Supervisor runs child goroutines, recovers panics, and restarts them
// according to policy.
type Supervisor struct {
	policy RestartPolicy
	wg     sync.WaitGroup
}

// NewSupervisor returns a Supervisor with the given restart policy.
func NewSupervisor(policy RestartPolicy) *Supervisor {
	return &Supervisor{policy: policy}
}

// Spawn starts run in a managed goroutine. The supervisor recovers panics
// and restarts per policy until either ctx is cancelled or MaxRestarts is
// exceeded.
func (s *Supervisor) Spawn(ctx context.Context, name string, run func(ctx context.Context) error) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		restarts := 0
		for {
			err := s.runOnce(ctx, run)
			if err == nil || ctx.Err() != nil {
				return
			}
			restarts++
			if s.policy.MaxRestarts > 0 && restarts > s.policy.MaxRestarts {
				return
			}
			if s.policy.Backoff > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(s.policy.Backoff):
				}
			}
			_ = name // available for future structured logging
		}
	}()
}

func (s *Supervisor) runOnce(ctx context.Context, run func(ctx context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("actor: child panic: %v", r)
		}
	}()
	return run(ctx)
}

// Wait blocks until every spawned child has exited.
func (s *Supervisor) Wait() { s.wg.Wait() }
