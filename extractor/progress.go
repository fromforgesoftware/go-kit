package extractor

import (
	"context"
	"encoding/json"
	"io"
	"sync/atomic"
	"time"
)

// Snapshot is a point-in-time view of a Progress meter.
type Snapshot struct {
	Name    string        `json:"name"`
	Total   int64         `json:"total"`
	Done    int64         `json:"done"`
	Errors  int64         `json:"errors"`
	Bytes   int64         `json:"bytes"`
	Started time.Time     `json:"started"`
	Elapsed time.Duration `json:"elapsed_ms"`
	ETA     time.Duration `json:"eta_ms"`
	Pct     float32       `json:"pct"`
}

// Progress is a goroutine-safe counter meter for long-running batches.
type Progress struct {
	name    string
	total   int64
	done    int64
	errors  int64
	bytes   int64
	started time.Time
}

// NewProgress constructs a Progress meter named `name` with `total`
// expected units.
func NewProgress(name string, total int64) *Progress {
	return &Progress{name: name, total: total, started: time.Now()}
}

// Inc adds `by` to the done count.
func (p *Progress) Inc(by int64) { atomic.AddInt64(&p.done, by) }

// IncBytes adds `by` to the byte counter (used by content / extraction
// pipelines that care about throughput).
func (p *Progress) IncBytes(by int64) { atomic.AddInt64(&p.bytes, by) }

// Error increments the error counter.
func (p *Progress) Error() { atomic.AddInt64(&p.errors, 1) }

// Snapshot captures the current state.
func (p *Progress) Snapshot() Snapshot {
	done := atomic.LoadInt64(&p.done)
	elapsed := time.Since(p.started)
	pct := float32(0)
	if p.total > 0 {
		pct = float32(done) / float32(p.total)
	}
	var eta time.Duration
	if done > 0 && p.total > done {
		eta = time.Duration(float64(elapsed) * float64(p.total-done) / float64(done))
	}
	return Snapshot{
		Name:    p.name,
		Total:   p.total,
		Done:    done,
		Errors:  atomic.LoadInt64(&p.errors),
		Bytes:   atomic.LoadInt64(&p.bytes),
		Started: p.started,
		Elapsed: elapsed,
		ETA:     eta,
		Pct:     pct,
	}
}

// Stream writes a JSON snapshot every `every` to `out` until ctx is
// cancelled. Suited for CI heartbeats.
func (p *Progress) Stream(ctx context.Context, every time.Duration, out io.Writer) error {
	if every <= 0 {
		every = time.Second
	}
	enc := json.NewEncoder(out)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if err := enc.Encode(p.Snapshot()); err != nil {
				return err
			}
		}
	}
}
