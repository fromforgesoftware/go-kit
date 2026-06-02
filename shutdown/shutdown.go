// Package shutdown coordinates a process's graceful shutdown — registered
// hooks run in LIFO order on signal (SIGINT/SIGTERM) or explicit Stop(),
// each with its own timeout, errors are aggregated rather than swallowed.
//
// Designed for plain `func main()` programs that don't use fx. fx-based
// services already get this for free via lifecycle hooks.
//
//	c := shutdown.New(shutdown.WithGraceWindow(30 * time.Second))
//	c.Register("http-server", server.Stop)
//	c.Register("db", db.Close)
//	if err := c.Run(ctx); err != nil {
//	    log.Fatal(err)
//	}
package shutdown

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Hook is invoked during shutdown. Implementations should respect ctx and
// return promptly when it's cancelled — the coordinator imposes per-hook
// timeouts.
type Hook func(ctx context.Context) error

type entry struct {
	name string
	fn   Hook
}

// Coordinator manages a stack of shutdown hooks.
type Coordinator struct {
	mu          sync.Mutex
	hooks       []entry
	hookTimeout time.Duration
	graceWindow time.Duration
	signals     []os.Signal
	logf        func(format string, args ...any)
	stopped     bool
	stopCh      chan struct{}
}

type Option func(*Coordinator)

// WithHookTimeout sets the per-hook timeout. Default: 10s.
func WithHookTimeout(d time.Duration) Option {
	return func(c *Coordinator) { c.hookTimeout = d }
}

// WithGraceWindow sets the overall grace budget. Hooks running past this
// are abandoned and Run returns with their timeout error. Default: 30s.
func WithGraceWindow(d time.Duration) Option {
	return func(c *Coordinator) { c.graceWindow = d }
}

// WithSignals overrides the signal set that triggers shutdown.
// Default: SIGINT + SIGTERM.
func WithSignals(sig ...os.Signal) Option {
	return func(c *Coordinator) { c.signals = sig }
}

// WithLogger wires a logging function called with shutdown lifecycle events.
// Defaults to a no-op.
func WithLogger(logf func(format string, args ...any)) Option {
	return func(c *Coordinator) { c.logf = logf }
}

// New constructs a Coordinator with the supplied options.
func New(opts ...Option) *Coordinator {
	c := &Coordinator{
		hookTimeout: 10 * time.Second,
		graceWindow: 30 * time.Second,
		signals:     []os.Signal{syscall.SIGINT, syscall.SIGTERM},
		logf:        func(string, ...any) {},
		stopCh:      make(chan struct{}),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Register adds a hook. Hooks run in LIFO order, so register cheap-to-stop
// things last (servers before databases, for example).
func (c *Coordinator) Register(name string, fn Hook) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hooks = append(c.hooks, entry{name: name, fn: fn})
}

// Stop triggers shutdown explicitly. Safe to call multiple times — only the
// first call has effect.
func (c *Coordinator) Stop() {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return
	}
	c.stopped = true
	close(c.stopCh)
	c.mu.Unlock()
}

// Run blocks until ctx is cancelled, a configured signal arrives, or Stop is
// called, then runs all registered hooks in LIFO order. Returns the joined
// error of any hooks that failed or timed out.
func (c *Coordinator) Run(ctx context.Context) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, c.signals...)
	defer signal.Stop(sigCh)

	select {
	case <-ctx.Done():
		c.logf("shutdown: ctx done (%v)", ctx.Err())
	case sig := <-sigCh:
		c.logf("shutdown: caught %s", sig)
	case <-c.stopCh:
		c.logf("shutdown: Stop() called")
	}

	return c.runHooks()
}

func (c *Coordinator) runHooks() error {
	c.mu.Lock()
	hooks := make([]entry, len(c.hooks))
	for i, h := range c.hooks {
		hooks[len(c.hooks)-1-i] = h // LIFO
	}
	c.mu.Unlock()

	overall, cancel := context.WithTimeout(context.Background(), c.graceWindow)
	defer cancel()

	var errs []error
	for _, h := range hooks {
		hookCtx, hookCancel := context.WithTimeout(overall, c.hookTimeout)
		started := time.Now()
		err := h.fn(hookCtx)
		hookCancel()
		elapsed := time.Since(started)
		if err != nil {
			c.logf("shutdown: hook %q failed after %s: %v", h.name, elapsed, err)
			errs = append(errs, fmt.Errorf("hook %q: %w", h.name, err))
			continue
		}
		c.logf("shutdown: hook %q ok (%s)", h.name, elapsed)
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}
