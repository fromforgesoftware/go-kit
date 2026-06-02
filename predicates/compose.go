package predicates

import "context"

// And returns a predicate that is true only when every child is true.
// Empty And is true.
func And(p ...Predicate) Predicate {
	return PredicateFunc(func(ctx context.Context, src Source) bool {
		for _, q := range p {
			if !q.Eval(ctx, src) {
				return false
			}
		}
		return true
	})
}

// Or returns a predicate that is true when any child is true. Empty Or
// is false.
func Or(p ...Predicate) Predicate {
	return PredicateFunc(func(ctx context.Context, src Source) bool {
		for _, q := range p {
			if q.Eval(ctx, src) {
				return true
			}
		}
		return false
	})
}

// Not inverts a predicate.
func Not(p Predicate) Predicate {
	return PredicateFunc(func(ctx context.Context, src Source) bool {
		return !p.Eval(ctx, src)
	})
}
