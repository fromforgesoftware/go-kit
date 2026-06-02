package predicates_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/predicates"
)

type ctxSrc struct {
	Level int
	Class string
}

func atLeastLevel(min int) predicates.Predicate {
	return predicates.PredicateFunc(func(_ context.Context, s predicates.Source) bool {
		return s.(ctxSrc).Level >= min
	})
}

func isClass(c string) predicates.Predicate {
	return predicates.PredicateFunc(func(_ context.Context, s predicates.Source) bool {
		return s.(ctxSrc).Class == c
	})
}

func TestComposeBasics(t *testing.T) {
	ctx := context.Background()
	src := ctxSrc{Level: 10, Class: "mage"}

	assert.True(t, atLeastLevel(5).Eval(ctx, src))
	assert.False(t, atLeastLevel(20).Eval(ctx, src))

	assert.True(t, predicates.And(atLeastLevel(5), isClass("mage")).Eval(ctx, src))
	assert.False(t, predicates.And(atLeastLevel(5), isClass("rogue")).Eval(ctx, src))

	assert.True(t, predicates.Or(atLeastLevel(20), isClass("mage")).Eval(ctx, src))
	assert.False(t, predicates.Or(atLeastLevel(20), isClass("rogue")).Eval(ctx, src))

	assert.False(t, predicates.Not(atLeastLevel(5)).Eval(ctx, src))
}

func TestComposeEmptyDefaults(t *testing.T) {
	ctx := context.Background()
	assert.True(t, predicates.And().Eval(ctx, ctxSrc{}))
	assert.False(t, predicates.Or().Eval(ctx, ctxSrc{}))
}

func TestDeMorgan(t *testing.T) {
	ctx := context.Background()
	cases := []ctxSrc{{Level: 1, Class: "a"}, {Level: 10, Class: "b"}, {Level: 20, Class: "c"}}

	for _, src := range cases {
		a := atLeastLevel(5)
		b := isClass("a")

		notAnd := predicates.Not(predicates.And(a, b)).Eval(ctx, src)
		orNots := predicates.Or(predicates.Not(a), predicates.Not(b)).Eval(ctx, src)
		assert.Equal(t, notAnd, orNots, "De Morgan: !(A&B) == !A|!B")

		notOr := predicates.Not(predicates.Or(a, b)).Eval(ctx, src)
		andNots := predicates.And(predicates.Not(a), predicates.Not(b)).Eval(ctx, src)
		assert.Equal(t, notOr, andNots, "De Morgan: !(A|B) == !A&!B")
	}
}

func TestRegistryBuild(t *testing.T) {
	r := predicates.NewRegistry()
	type levelParams struct {
		Min int `json:"min"`
	}
	predicates.RegisterTyped(r, "at_least_level", func(p levelParams) (predicates.Predicate, error) {
		return atLeastLevel(p.Min), nil
	})
	type classParams struct {
		Name string `json:"name"`
	}
	predicates.RegisterTyped(r, "is_class", func(p classParams) (predicates.Predicate, error) {
		return isClass(p.Name), nil
	})

	spec := predicates.Spec{
		Type: "and",
		Sub: []predicates.Spec{
			{Type: "at_least_level", Params: json.RawMessage(`{"min":5}`)},
			{Type: "is_class", Params: json.RawMessage(`{"name":"mage"}`)},
		},
	}
	p, err := predicates.Build(r, spec)
	require.NoError(t, err)
	assert.True(t, p.Eval(context.Background(), ctxSrc{Level: 10, Class: "mage"}))
	assert.False(t, p.Eval(context.Background(), ctxSrc{Level: 4, Class: "mage"}))
}

func TestRegistryRejectsUnknown(t *testing.T) {
	r := predicates.NewRegistry()
	_, err := predicates.Build(r, predicates.Spec{Type: "ghost"})
	assert.ErrorIs(t, err, predicates.ErrUnknownType)
}
