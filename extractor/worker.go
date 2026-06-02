package extractor

import (
	"context"
	"errors"
	"sync"
)

// Recovery tells the worker how to handle a per-unit error.
type Recovery uint8

const (
	// Continue keeps processing the unit (the error is logged via
	// OnError but not retried — the caller decides).
	Continue Recovery = iota
	// Skip drops the unit and moves on.
	Skip
	// Abort cancels the worker pool with the unit's error.
	Abort
)

// WorkerConfig configures a Worker.
type WorkerConfig[T, R any] struct {
	// Concurrency is the max number of in-flight units. Defaults to 1.
	Concurrency int
	// Process is the per-unit work. Required.
	Process func(ctx context.Context, unit T) (R, error)
	// OnResult fires after a successful Process; receives the unit
	// and its result. Optional.
	OnResult func(unit T, result R)
	// OnError fires when Process returns an error and decides how to
	// recover. Defaults to Continue (log + carry on) when nil.
	OnError func(unit T, err error) Recovery
}

// Errors surfaced by Worker.
var (
	ErrAborted         = errors.New("extractor: worker aborted by OnError policy")
	ErrProcessRequired = errors.New("extractor: WorkerConfig.Process is required")
)

// Worker is a bounded-concurrency processor for a slice of units.
type Worker[T, R any] struct {
	cfg WorkerConfig[T, R]
}

// NewWorker constructs a Worker.
func NewWorker[T, R any](cfg WorkerConfig[T, R]) (*Worker[T, R], error) {
	if cfg.Process == nil {
		return nil, ErrProcessRequired
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.OnError == nil {
		cfg.OnError = func(_ T, _ error) Recovery { return Continue }
	}
	return &Worker[T, R]{cfg: cfg}, nil
}

// Run processes every unit in `units` concurrently up to Concurrency.
// Returns ctx.Err() on cancellation, or the first error wrapped in
// ErrAborted if OnError requested Abort.
func (w *Worker[T, R]) Run(ctx context.Context, units []T) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, w.cfg.Concurrency)
	var (
		wg      sync.WaitGroup
		firstMu sync.Mutex
		first   error
	)
	setFirst := func(err error) {
		firstMu.Lock()
		defer firstMu.Unlock()
		if first == nil {
			first = err
		}
	}

	for _, unit := range units {
		select {
		case <-ctx.Done():
			wg.Wait()
			if first != nil {
				return first
			}
			return ctx.Err()
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func(u T) {
			defer wg.Done()
			defer func() { <-sem }()
			result, err := w.cfg.Process(ctx, u)
			if err != nil {
				switch w.cfg.OnError(u, err) {
				case Skip, Continue:
					return
				case Abort:
					setFirst(errors.Join(ErrAborted, err))
					cancel()
				}
				return
			}
			if w.cfg.OnResult != nil {
				w.cfg.OnResult(u, result)
			}
		}(unit)
	}

	wg.Wait()
	if first != nil {
		return first
	}
	return ctx.Err()
}
