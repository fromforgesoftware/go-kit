package fsm

import (
	"errors"
	"fmt"
)

// ErrInvalidSpec is returned when a Spec fails construction-time validation.
var ErrInvalidSpec = errors.New("fsm: invalid spec")

// Validate checks a Spec for common errors: unreachable states, transitions
// targeting states never entered, and duplicate (From, Event) pairs without
// guards (which would make selection non-deterministic).
func Validate[S, E comparable](spec Spec[S, E]) error {
	if len(spec.Transitions) == 0 {
		return fmt.Errorf("%w: no transitions", ErrInvalidSpec)
	}
	seenFrom := map[S]bool{spec.Initial: true}
	type key struct {
		s S
		e E
	}
	guardless := map[key]int{}
	for _, t := range spec.Transitions {
		seenFrom[t.From] = true
		seenFrom[t.To] = true
		if t.Guard == nil {
			k := key{t.From, t.Event}
			guardless[k]++
			if guardless[k] > 1 {
				return fmt.Errorf("%w: duplicate guardless transition for state-event pair", ErrInvalidSpec)
			}
		}
	}
	// All OnEnter/OnExit keys must reference reachable states.
	for s := range spec.OnEnter {
		if !seenFrom[s] {
			return fmt.Errorf("%w: OnEnter references unreachable state", ErrInvalidSpec)
		}
	}
	for s := range spec.OnExit {
		if !seenFrom[s] {
			return fmt.Errorf("%w: OnExit references unreachable state", ErrInvalidSpec)
		}
	}
	return nil
}
