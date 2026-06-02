package outbox

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/fromforgesoftware/go-kit/monitoring"
)

// Dispatcher is the shared core that turns a batch of Messages into
// Handler invocations. Worker (long-running loop) and Drainer (single
// pass) both delegate here so retry/backoff/error handling stays in
// one place.
type Dispatcher struct {
	repo        Repository
	handlers    map[string]Handler
	monitor     monitoring.Monitor
	maxAttempts int
	baseBackoff time.Duration
}

// Config is the shared knobs Worker + Drainer expose.
type Config struct {
	// BatchSize caps how many messages a single Claim returns. Default 50.
	BatchSize int
	// MaxAttempts: after this many failures a row stays put with the
	// last error and isn't reclaimed. Default 10.
	MaxAttempts int
	// BaseBackoff is the unit of exponential backoff: retry_at = now +
	// BaseBackoff * 2^(attempts-1), capped at 1h. Default 5s.
	BaseBackoff time.Duration
	// PollInterval is how often the Worker loop polls. Drainer ignores
	// this (single pass). Default 5s.
	PollInterval time.Duration
}

func (c Config) withDefaults() Config {
	if c.BatchSize <= 0 {
		c.BatchSize = 50
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 10
	}
	if c.BaseBackoff <= 0 {
		c.BaseBackoff = 5 * time.Second
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 5 * time.Second
	}
	return c
}

func newDispatcher(repo Repository, handlers map[string]Handler, monitor monitoring.Monitor, cfg Config) *Dispatcher {
	cfg = cfg.withDefaults()
	return &Dispatcher{
		repo:        repo,
		handlers:    handlers,
		monitor:     monitor,
		maxAttempts: cfg.MaxAttempts,
		baseBackoff: cfg.BaseBackoff,
	}
}

// drainOnce claims a single batch and dispatches each message. Returns
// the number of messages successfully handled and the first
// non-handler error encountered (claim/mark failures). Handler errors
// don't return — the row is just marked failed.
func (d *Dispatcher) drainOnce(ctx context.Context, batch int) (int, error) {
	msgs, err := d.repo.Claim(ctx, batch)
	if err != nil {
		return 0, fmt.Errorf("outbox: claim: %w", err)
	}
	if len(msgs) == 0 {
		return 0, nil
	}

	ok := 0
	for _, m := range msgs {
		h, found := d.handlers[m.Kind]
		if !found {
			d.markFailed(ctx, m, fmt.Errorf("no handler registered for kind %q", m.Kind))
			continue
		}
		if err := h.Handle(ctx, m); err != nil {
			d.markFailed(ctx, m, err)
			continue
		}
		if err := d.repo.MarkDone(ctx, m.ID); err != nil {
			d.monitor.Logger().ErrorContext(ctx, "outbox: mark done failed",
				"id", m.ID, "kind", m.Kind, "err", err.Error())
			continue
		}
		ok++
	}
	return ok, nil
}

func (d *Dispatcher) markFailed(ctx context.Context, m Message, handlerErr error) {
	attempts := m.Attempts + 1
	retryAt := time.Now().Add(d.backoff(attempts))
	if attempts >= d.maxAttempts {
		// Park the row: bump retry_at far enough that normal Claim
		// queries skip it forever. Operators inspect the table for
		// rows with attempts >= MaxAttempts.
		retryAt = time.Now().Add(100 * 365 * 24 * time.Hour)
		d.monitor.Logger().ErrorContext(ctx, "outbox: dead-lettering after max attempts",
			"id", m.ID, "kind", m.Kind, "attempts", attempts, "err", handlerErr.Error())
	} else {
		d.monitor.Logger().WarnContext(ctx, "outbox: handler failed, will retry",
			"id", m.ID, "kind", m.Kind, "attempts", attempts, "err", handlerErr.Error())
	}
	if err := d.repo.MarkFailed(ctx, m.ID, handlerErr, retryAt); err != nil {
		d.monitor.Logger().ErrorContext(ctx, "outbox: mark failed failed",
			"id", m.ID, "kind", m.Kind, "err", err.Error())
	}
}

func (d *Dispatcher) backoff(attempts int) time.Duration {
	// 2^(attempts-1) * baseBackoff, capped at 1h.
	exp := math.Pow(2, float64(attempts-1))
	dur := time.Duration(exp) * d.baseBackoff
	if dur > time.Hour {
		dur = time.Hour
	}
	return dur
}

// Worker keeps a long-running loop that polls the outbox at
// PollInterval. Use this in local dev (`make dev`) or on a process
// that gets always-on CPU. On Cloud Run with default CPU throttling,
// prefer Drainer + Cloud Scheduler.
type Worker struct {
	*Dispatcher
	cfg Config
}

func NewWorker(repo Repository, handlers map[string]Handler, monitor monitoring.Monitor, cfg Config) *Worker {
	cfg = cfg.withDefaults()
	return &Worker{Dispatcher: newDispatcher(repo, handlers, monitor, cfg), cfg: cfg}
}

// Run blocks until ctx is cancelled. Drains one batch per
// PollInterval; if a batch returns < BatchSize messages, sleeps the
// full interval; if it returns a full batch, immediately polls again
// (catch-up mode).
func (w *Worker) Run(ctx context.Context) error {
	for {
		n, err := w.drainOnce(ctx, w.cfg.BatchSize)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			w.monitor.Logger().ErrorContext(ctx, "outbox: drain pass failed", "err", err.Error())
		}
		if n == w.cfg.BatchSize {
			// Likely more work — go again immediately.
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(w.cfg.PollInterval):
		}
	}
}

// Drainer is the Cloud Run Job shape: one pass over the outbox, then
// exits. Returns when there's nothing left to claim OR the batch came
// back empty. Idempotent and re-runnable: Cloud Scheduler triggers
// every N seconds; each invocation drains whatever's pending.
type Drainer struct {
	*Dispatcher
	cfg Config
}

func NewDrainer(repo Repository, handlers map[string]Handler, monitor monitoring.Monitor, cfg Config) *Drainer {
	cfg = cfg.withDefaults()
	return &Drainer{Dispatcher: newDispatcher(repo, handlers, monitor, cfg), cfg: cfg}
}

// Drain processes batches until Claim returns empty or ctx is done.
// Returns the total messages successfully handled across all batches
// in this invocation.
func (d *Drainer) Drain(ctx context.Context) (int, error) {
	total := 0
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := d.drainOnce(ctx, d.cfg.BatchSize)
		if err != nil {
			return total, err
		}
		total += n
		if n < d.cfg.BatchSize {
			return total, nil
		}
	}
}
