package actor

import (
	"context"
	"time"
)

// Behavior defines an Actor's lifecycle and message handling. State is
// owned by the Actor's goroutine and never escapes.
type Behavior[M, S any] interface {
	Init(ctx context.Context) (S, error)
	Handle(ctx context.Context, state *S, msg M) error
	Tick(ctx context.Context, state *S, now time.Time) error
	Close(ctx context.Context, state *S) error
}

// Option customises an Actor.
type Option func(*config)

type config struct {
	mailboxCap int
	tickEvery  time.Duration
}

// WithMailboxCap overrides the default mailbox capacity (256).
func WithMailboxCap(n int) Option {
	return func(c *config) { c.mailboxCap = n }
}

// WithTickEvery enables a periodic Tick call at the given interval.
func WithTickEvery(d time.Duration) Option {
	return func(c *config) { c.tickEvery = d }
}

// Actor owns a goroutine, a mailbox, and Behavior-managed state.
type Actor[M, S any] struct {
	behavior Behavior[M, S]
	cfg      config
	mb       *Mailbox[M]
}

// New constructs an Actor; call Run to start it.
func New[M, S any](b Behavior[M, S], opts ...Option) *Actor[M, S] {
	cfg := config{mailboxCap: 256}
	for _, o := range opts {
		o(&cfg)
	}
	return &Actor[M, S]{behavior: b, cfg: cfg, mb: NewMailbox[M](cfg.mailboxCap)}
}

// Send delivers a message to the actor's mailbox.
func (a *Actor[M, S]) Send(ctx context.Context, msg M) error {
	return a.mb.Send(ctx, msg)
}

// Run drives the actor loop until ctx is cancelled. Returns the first
// error from Init/Handle/Tick/Close, or ctx.Err() on cancellation.
func (a *Actor[M, S]) Run(ctx context.Context) error {
	state, err := a.behavior.Init(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = a.behavior.Close(ctx, &state) }()

	var tickC <-chan time.Time
	if a.cfg.tickEvery > 0 {
		t := time.NewTicker(a.cfg.tickEvery)
		defer t.Stop()
		tickC = t.C
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case now := <-tickC:
			if err := a.behavior.Tick(ctx, &state, now); err != nil {
				return err
			}
		case msg := <-a.mb.ch:
			if err := a.behavior.Handle(ctx, &state, msg); err != nil {
				return err
			}
		}
	}
}
