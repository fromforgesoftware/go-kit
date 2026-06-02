// Package cron is a tiny in-process recurring-job scheduler. Each job runs
// on an independent goroutine with a configurable interval, optional jitter,
// and panic recovery. For complex scheduling needs (cron expressions across
// instances, persistent queues, exactly-once) use temporal or a dedicated
// scheduler — this package is the boilerplate-killer for the common
// "every minute / every hour" workloads.
//
//	sched := cron.New(cron.WithLogger(log))
//	sched.Every(time.Minute, "cleanup-temp", func(ctx context.Context) error { ... })
//	sched.Every(5*time.Minute, "refresh-cache", func(ctx context.Context) error { ... })
//	sched.Start(ctx)
//	defer sched.Stop()
package cron

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"
)

// JobFunc is a single execution of a recurring job.
type JobFunc func(ctx context.Context) error

// Scheduler owns a set of recurring jobs.
type Scheduler struct {
	mu      sync.Mutex
	jobs    []*jobEntry
	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	logf    func(format string, args ...any)
	jitter  float64 // 0–1 fraction
	timeout time.Duration
}

type jobEntry struct {
	name     string
	interval time.Duration
	fn       JobFunc
	runs     int64
	failures int64
	lastErr  error
	mu       sync.Mutex
}

type Option func(*Scheduler)

// WithLogger wires a logging function called with job lifecycle events.
func WithLogger(logf func(format string, args ...any)) Option {
	return func(s *Scheduler) { s.logf = logf }
}

// WithJitter spreads job firing by up to `frac * interval` per run, to
// avoid thundering-herd patterns when many jobs share an interval.
// Default: 0.1 (10%).
func WithJitter(frac float64) Option {
	return func(s *Scheduler) {
		if frac < 0 {
			frac = 0
		}
		if frac > 1 {
			frac = 1
		}
		s.jitter = frac
	}
}

// WithJobTimeout caps how long any single job invocation can run. Default
// unset (no timeout — relies on Stop or ctx cancellation).
func WithJobTimeout(d time.Duration) Option {
	return func(s *Scheduler) { s.timeout = d }
}

// New constructs a Scheduler.
func New(opts ...Option) *Scheduler {
	s := &Scheduler{
		logf:   func(string, ...any) {},
		jitter: 0.1,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Every registers a recurring job. Panics if called after Start.
func (s *Scheduler) Every(interval time.Duration, name string, fn JobFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		panic("cron: Every() called after Start()")
	}
	if interval <= 0 {
		panic("cron: interval must be > 0")
	}
	s.jobs = append(s.jobs, &jobEntry{name: name, interval: interval, fn: fn})
}

// Start launches every registered job's loop. Returns immediately; the
// scheduler runs until ctx is cancelled or Stop is called.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	jobs := s.jobs
	s.mu.Unlock()

	for _, j := range jobs {
		s.wg.Add(1)
		go s.runJob(runCtx, j)
	}
}

// Stop signals all jobs to stop and waits for them to finish their
// current invocation. Safe to call multiple times.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

// Stats reports per-job execution counters.
func (s *Scheduler) Stats() []JobStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]JobStats, 0, len(s.jobs))
	for _, j := range s.jobs {
		j.mu.Lock()
		out = append(out, JobStats{
			Name:      j.name,
			Interval:  j.interval,
			Runs:      j.runs,
			Failures:  j.failures,
			LastError: errorMessage(j.lastErr),
		})
		j.mu.Unlock()
	}
	return out
}

// JobStats is a point-in-time snapshot of a job's counters.
type JobStats struct {
	Name      string
	Interval  time.Duration
	Runs      int64
	Failures  int64
	LastError string
}

func (s *Scheduler) runJob(ctx context.Context, j *jobEntry) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.nextDelay(j.interval)):
			s.invoke(ctx, j)
		}
	}
}

func (s *Scheduler) invoke(ctx context.Context, j *jobEntry) {
	jobCtx := ctx
	var cancel context.CancelFunc
	if s.timeout > 0 {
		jobCtx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	start := time.Now()

	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		return j.fn(jobCtx)
	}()

	j.mu.Lock()
	j.runs++
	if err != nil {
		j.failures++
		j.lastErr = err
	} else {
		j.lastErr = nil
	}
	j.mu.Unlock()

	if err != nil {
		s.logf("cron: job %q failed after %s: %v", j.name, time.Since(start), err)
	} else {
		s.logf("cron: job %q ok (%s)", j.name, time.Since(start))
	}
}

func (s *Scheduler) nextDelay(interval time.Duration) time.Duration {
	if s.jitter <= 0 {
		return interval
	}
	max := float64(interval) * s.jitter
	offset := time.Duration(rand.Float64()*max) - time.Duration(max/2)
	return interval + offset
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// ErrAlreadyStarted is returned when Start is called twice.
var ErrAlreadyStarted = errors.New("cron: scheduler already started")
