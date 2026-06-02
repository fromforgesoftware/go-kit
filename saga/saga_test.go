package saga_test

import (
	"testing"

	"github.com/fromforgesoftware/go-kit/saga"
	"github.com/stretchr/testify/assert"
)

func sagaWithChildren[T any](t *testing.T, nodes ...saga.Node[T]) saga.Node[T] {
	t.Helper()

	return saga.NewNode(
		saga.WithChilds(nodes...),
	)
}

func sagaWithChildrenSteps[T any](t *testing.T, steps ...saga.StepRunner[T]) saga.Node[T] {
	t.Helper()

	children := make([]saga.Node[T], len(steps))
	for i := range steps {
		children[i] = saga.NewNode(saga.WithStep(steps[i]))
	}

	return sagaWithChildren(t, children...)
}

func TestRunSagaReturnsErr(t *testing.T) {
	tests := []struct {
		name         string
		initialValue int
		node         saga.Node[int]
		want         int
		wantErr      error
	}{
		{
			name: `
				given all the steps are non-compensable,
				and given one of the steps returns an error,
				return the value when it failed and error
			`,
			initialValue: 0,
			node:         sagaWithChildrenSteps(t, AddOneStep(), ErrorStep(), AddOneStep()),
			want:         1,
			wantErr:      assert.AnError,
		},
		{
			name: `
				given a mix of compensable and non-compensable steps,
				and given one of the steps returns an error,
				return the value compensated and error
			`,
			initialValue: 0,
			node: sagaWithChildren(t,
				saga.NewNode(saga.WithStep(AddOneStep()), saga.WithCompensation(RemoveOneStep())),
				saga.NewNode(saga.WithStep(AddOneStep()), saga.WithCompensation(RemoveOneStep())),
				saga.NewNode(saga.WithStep(ErrorStep())),
			),
			want:    0,
			wantErr: assert.AnError,
		},
		{
			name: `
				given a mix of compensable and non-compensable steps,
				and given one of the compansable steps returns an error,
				return the value compensated without compensating the failing step
				and return the error
			`,
			initialValue: 0,
			node: sagaWithChildren(t,
				saga.NewNode(saga.WithStep(AddOneStep()), saga.WithCompensation(RemoveOneStep())),
				saga.NewNode(saga.WithStep(AddOneStep())),
				saga.NewNode(saga.WithStep(ErrorStep()), saga.WithCompensation(RemoveOneStep())),
			),
			want:    1,
			wantErr: assert.AnError,
		},
		{
			name: `
				given a nested tree of steps,
				and given that a step in the most nested condition returns an error,
				and given all the conditions are true,
				return the value by compensating the steps that had already been executed,
				and return the error
			`,
			initialValue: 0,
			node: sagaWithChildren(t,
				saga.NewNode(saga.WithStep(AddOneStep()), saga.WithCompensation(RemoveOneStep())),
				saga.NewNode(
					saga.WithCondition(func(_ int) bool { return true }),
					saga.WithChilds(
						saga.NewNode(saga.WithStep(AddOneStep()), saga.WithCompensation(RemoveOneStep())),
						saga.NewNode(
							saga.WithCondition(func(_ int) bool { return true }),
							saga.WithChilds(
								saga.NewNode(saga.WithStep(AddOneStep())),
								saga.NewNode(saga.WithStep(ErrorStep()), saga.WithCompensation(RemoveOneStep())),
							),
						),
						saga.NewNode(saga.WithStep(AddOneStep())),
					),
				),
				saga.NewNode(saga.WithStep(AddOneStep()), saga.WithCompensation(RemoveOneStep())),
			),
			want:    1,
			wantErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := saga.New(tt.node)
			got, err := s.Run(t.Context(), tt.initialValue)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func TestRunSagaNoError(t *testing.T) {
	tests := []struct {
		name         string
		initialValue int
		node         saga.Node[int]
		want         int
	}{
		{
			name: `
				given all the steps are non-compensable,
				return the result of applying all the steps
			`,
			initialValue: 0,
			node:         sagaWithChildrenSteps(t, AddOneStep(), AddOneStep()),
			want:         2,
		},
		{
			name: `
				given a mix of compensable and non-compensable steps,
				return the result of applying all the steps
			`,
			initialValue: 0,
			node: sagaWithChildren(t,
				saga.NewNode(saga.WithStep(AddOneStep())),
				saga.NewNode(saga.WithStep(AddOneStep()), saga.WithCompensation(RemoveOneStep())),
				saga.NewNode(saga.WithStep(AddOneStep())),
			),
			want: 3,
		},
		{
			name: `
				given a nested tree of steps,
				and given all the conditions return true,
				return the value by executing all the leaf nodes in the tree
			`,
			initialValue: 0,
			node: sagaWithChildren(t,
				saga.NewNode(saga.WithStep(AddOneStep()), saga.WithCompensation(RemoveOneStep())),
				saga.NewNode(
					saga.WithCondition(func(_ int) bool { return true }),
					saga.WithChilds(
						saga.NewNode(saga.WithStep(AddOneStep()), saga.WithCompensation(RemoveOneStep())),
						saga.NewNode(
							saga.WithCondition(func(_ int) bool { return true }),
							saga.WithChilds(
								saga.NewNode(saga.WithStep(AddOneStep()), saga.WithCompensation(RemoveOneStep())),
								saga.NewNode(saga.WithStep(AddOneStep())),
							),
						),
						saga.NewNode(saga.WithStep(AddOneStep())),
					),
				),
				saga.NewNode(saga.WithStep(AddOneStep()), saga.WithCompensation(RemoveOneStep())),
			),
			want: 6,
		},
		{
			name: `
				given a nested tree of steps,
				and given the most nested condition return false,
				return the value by executing all the leaf nodes except from the ones in the most nested condition
			`,
			initialValue: 0,
			node: sagaWithChildren(t,
				saga.NewNode(saga.WithStep(AddOneStep()), saga.WithCompensation(RemoveOneStep())),
				saga.NewNode(
					saga.WithCondition(func(_ int) bool { return true }),
					saga.WithChilds(
						saga.NewNode(saga.WithStep(AddOneStep()), saga.WithCompensation(RemoveOneStep())),
						saga.NewNode(
							saga.WithCondition(func(_ int) bool { return false }),
							saga.WithChilds(
								saga.NewNode(saga.WithStep(AddOneStep()), saga.WithCompensation(RemoveOneStep())),
								saga.NewNode(saga.WithStep(AddOneStep())),
							),
						),
						saga.NewNode(saga.WithStep(AddOneStep())),
					),
				),
				saga.NewNode(saga.WithStep(AddOneStep()), saga.WithCompensation(RemoveOneStep())),
			),
			want: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := saga.New(tt.node)
			got, err := s.Run(t.Context(), tt.initialValue)
			assert.Equal(t, tt.want, got)
			assert.NoError(t, err)
		})
	}
}
