package predicates

import "context"

// Source is the marker interface for a predicate's evaluation context.
// Consumers define their own concrete types.
type Source interface{}

// Predicate evaluates a Source and returns true if satisfied.
type Predicate interface {
	Eval(ctx context.Context, src Source) bool
}

// PredicateFunc adapts a function to the Predicate interface.
type PredicateFunc func(ctx context.Context, src Source) bool

func (f PredicateFunc) Eval(ctx context.Context, src Source) bool {
	return f(ctx, src)
}
