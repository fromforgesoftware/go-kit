package saga_test

import (
	"context"
	"testing"

	"github.com/fromforgesoftware/go-kit/saga"
	"github.com/stretchr/testify/assert"
)

func AddOneStep() saga.StepRunner[int] {
	return func(_ context.Context, req int) (int, error) {
		return req + 1, nil
	}
}

func RemoveOneStep() saga.StepRunner[int] {
	return func(_ context.Context, req int) (int, error) {
		return req - 1, nil
	}
}

func ErrorStep() saga.StepRunner[int] {
	return func(_ context.Context, req int) (int, error) {
		return req, assert.AnError
	}
}

func ReturnOneStep() saga.StepRunner[int] {
	return func(_ context.Context, req int) (int, error) {
		return 1, nil
	}
}

func AddResults() saga.ResultMerger[int] {
	return func(ctx context.Context, req []int) (int, error) {
		total := 0
		for _, r := range req {
			total += r
		}
		return total, nil
	}
}

func TestSerialStep(t *testing.T) {
	t.Run("if one of the steps returns an error, return zero value and error", func(t *testing.T) {
		got, err := saga.SerialSteps(ErrorStep(), AddOneStep())(t.Context(), 0)
		assert.Zero(t, got)
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("if all the steps return no error, return the result of applying all the steps", func(t *testing.T) {
		got, err := saga.SerialSteps(AddOneStep(), AddOneStep())(t.Context(), 0)
		assert.Equal(t, 2, got)
		assert.NoError(t, err)
	})
}

func TestParallelStep(t *testing.T) {
	t.Run("if one of the steps returns an error, return zero value and error", func(t *testing.T) {
		got, err := saga.ParallelSteps(AddResults(), ReturnOneStep(), ErrorStep())(t.Context(), 0)
		assert.Zero(t, got)
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("if all the steps return no error, return the result of applying all the steps", func(t *testing.T) {
		got, err := saga.ParallelSteps(
			AddResults(),
			ReturnOneStep(),
			ReturnOneStep(),
			ReturnOneStep(),
		)(t.Context(), 0)
		assert.Equal(t, 3, got)
		assert.NoError(t, err)
	})
}
