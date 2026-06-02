// Package saga provides a simple way to define a step in a sequence of steps.
package saga

import (
	"context"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

type (
	StepRunner[T any]   func(ctx context.Context, req T) (T, error)
	ResultMerger[T any] func(ctx context.Context, req []T) (T, error)
)

// SerialSteps executes the steps in serial. Use it when you want to execute steps in serial.
func SerialSteps[T any](steps ...StepRunner[T]) StepRunner[T] {
	return func(ctx context.Context, req T) (T, error) {
		var zero T
		var err error
		for _, step := range steps {
			req, err = step(ctx, req)
			if err != nil {
				return zero, err
			}
		}
		return req, nil
	}
}

// ParallelSteps executes the steps in parallel. Use it when you do not need the result of one to execute the next one.
func ParallelSteps[T any](mergeStep ResultMerger[T], steps ...StepRunner[T]) StepRunner[T] {
	return func(ctx context.Context, req T) (T, error) {
		var zero T
		errs, ctx := errgroup.WithContext(ctx)
		out := make(chan T)

		numSteps := int32(len(steps))
		for i := range steps {
			ii := i // to avoid capturing loop value in closure
			errs.Go(func() error {
				defer func() {
					if atomic.AddInt32(&numSteps, -1) == 0 {
						close(out)
					}
				}()

				res, err := steps[ii](ctx, req)
				if err != nil {
					return err
				}
				out <- res
				return nil
			})
		}

		reqs := make([]T, len(steps))
		errs.Go(func() error {
			for elem := range out {
				reqs = append(reqs, elem)
			}
			return nil
		})

		err := errs.Wait()
		if err != nil {
			return zero, err
		}

		return mergeStep(ctx, reqs)
	}
}
