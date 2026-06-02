package extractor

import (
	"context"
	"errors"
)

// PipelineConfig wires a worker, optional progress meter, optional
// checkpoint, and the function that derives a stable unit id used
// for checkpointing.
type PipelineConfig[T, R any] struct {
	Worker     WorkerConfig[T, R]
	Progress   *Progress
	Checkpoint Checkpoint
	// UnitID derives the checkpoint key for a unit. Required when
	// Checkpoint is non-nil.
	UnitID func(T) string
}

// ErrUnitIDRequired is returned by NewPipeline when a Checkpoint is
// configured but UnitID is nil.
var ErrUnitIDRequired = errors.New("extractor: PipelineConfig.UnitID is required when Checkpoint is set")

// Pipeline composes worker + progress + checkpoint into one entry.
type Pipeline[T, R any] struct {
	cfg    PipelineConfig[T, R]
	worker *Worker[T, R]
}

// NewPipeline constructs a Pipeline.
func NewPipeline[T, R any](cfg PipelineConfig[T, R]) (*Pipeline[T, R], error) {
	if cfg.Checkpoint != nil && cfg.UnitID == nil {
		return nil, ErrUnitIDRequired
	}
	// Wrap the user's Process to integrate progress + checkpoint.
	userProcess := cfg.Worker.Process
	userOnError := cfg.Worker.OnError
	cfg.Worker.Process = func(ctx context.Context, unit T) (R, error) {
		var zero R
		if cfg.Checkpoint != nil {
			done, err := cfg.Checkpoint.Done(cfg.UnitID(unit))
			if err != nil {
				return zero, err
			}
			if done {
				return zero, errAlreadyDone
			}
		}
		return userProcess(ctx, unit)
	}
	cfg.Worker.OnError = func(unit T, err error) Recovery {
		if errors.Is(err, errAlreadyDone) {
			return Skip
		}
		if cfg.Progress != nil {
			cfg.Progress.Error()
		}
		if userOnError != nil {
			return userOnError(unit, err)
		}
		return Continue
	}
	userOnResult := cfg.Worker.OnResult
	cfg.Worker.OnResult = func(unit T, r R) {
		if cfg.Progress != nil {
			cfg.Progress.Inc(1)
		}
		if cfg.Checkpoint != nil {
			_ = cfg.Checkpoint.Mark(cfg.UnitID(unit))
		}
		if userOnResult != nil {
			userOnResult(unit, r)
		}
	}
	w, err := NewWorker(cfg.Worker)
	if err != nil {
		return nil, err
	}
	return &Pipeline[T, R]{cfg: cfg, worker: w}, nil
}

// Run processes every unit, skipping ones the Checkpoint already
// records as done. Progress is updated per successful unit.
func (p *Pipeline[T, R]) Run(ctx context.Context, units []T) error {
	return p.worker.Run(ctx, units)
}

// errAlreadyDone is the internal sentinel that triggers Skip when the
// checkpoint says a unit is already complete.
var errAlreadyDone = errors.New("extractor: unit already checkpointed")
