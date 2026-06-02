package ratelimit

import (
	"context"
	"time"
)

type limiter struct {
	store Store
	now   func() time.Time
}

// Option configures a Limiter.
type Option func(*limiter)

// WithClock overrides the time source (tests).
func WithClock(now func() time.Time) Option {
	return func(l *limiter) { l.now = now }
}

// New builds a Limiter over the given Store (use NewInMemoryStore for the
// default process-local backend).
func New(store Store, opts ...Option) Limiter {
	l := &limiter{store: store, now: time.Now}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

func (l *limiter) Allow(ctx context.Context, key string, policy Policy) (Result, error) {
	return l.AllowN(ctx, key, policy, 1)
}

func (l *limiter) AllowN(ctx context.Context, key string, policy Policy, cost int) (Result, error) {
	return l.store.Take(ctx, key, policy, cost, l.now())
}
