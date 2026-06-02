package predicates

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// ErrUnknownType is returned by Build when a Spec.Type is not registered.
var ErrUnknownType = errors.New("predicates: unknown predicate type")

// Registry holds factory functions keyed by predicate-type name. Factories
// receive the raw JSON params bytes so they can decode into their typed
// param struct.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]func(params json.RawMessage) (Predicate, error)
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]func(params json.RawMessage) (Predicate, error))}
}

// Register binds a factory to a type name. Factories that expect typed
// params can use the generic helper RegisterTyped instead.
func (r *Registry) Register(name string, fn func(params json.RawMessage) (Predicate, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = fn
}

// RegisterTyped binds a factory whose params decode into T. The wrapper
// handles json.Unmarshal so the user-supplied function takes T directly.
func RegisterTyped[T any](r *Registry, name string, fn func(params T) (Predicate, error)) {
	r.Register(name, func(raw json.RawMessage) (Predicate, error) {
		var p T
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, fmt.Errorf("predicates: decode params for %q: %w", name, err)
			}
		}
		return fn(p)
	})
}

// Spec is a serialisable predicate description: a leaf has Type + Params;
// composite nodes have Type "and" / "or" / "not" and Sub children.
type Spec struct {
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params,omitempty"`
	Sub    []Spec          `json:"sub,omitempty"`
}

// Build materialises a Spec into a Predicate using the registry. "and",
// "or", "not" are reserved composite types and never look at the registry.
func Build(r *Registry, spec Spec) (Predicate, error) {
	switch spec.Type {
	case "and":
		children, err := buildChildren(r, spec.Sub)
		if err != nil {
			return nil, err
		}
		return And(children...), nil
	case "or":
		children, err := buildChildren(r, spec.Sub)
		if err != nil {
			return nil, err
		}
		return Or(children...), nil
	case "not":
		if len(spec.Sub) != 1 {
			return nil, fmt.Errorf("predicates: 'not' requires exactly one child, got %d", len(spec.Sub))
		}
		child, err := Build(r, spec.Sub[0])
		if err != nil {
			return nil, err
		}
		return Not(child), nil
	}
	r.mu.RLock()
	fn, ok := r.factories[spec.Type]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownType, spec.Type)
	}
	return fn(spec.Params)
}

func buildChildren(r *Registry, subs []Spec) ([]Predicate, error) {
	out := make([]Predicate, 0, len(subs))
	for i, s := range subs {
		p, err := Build(r, s)
		if err != nil {
			return nil, fmt.Errorf("predicates: build child %d: %w", i, err)
		}
		out = append(out, p)
	}
	return out, nil
}
