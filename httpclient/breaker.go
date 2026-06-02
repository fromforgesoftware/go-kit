package httpclient

import (
	"errors"
	"sync"
	"time"
)

// State is one of:
//   - Closed: requests flow normally
//   - Open: trip threshold reached, requests fail fast until cooldown
//   - HalfOpen: probe state — one request is allowed to test recovery
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ErrBreakerOpen is returned by the client when the circuit is open.
var ErrBreakerOpen = errors.New("httpclient: circuit breaker open")

// Breaker is a small failure-rate-based circuit breaker.
type Breaker struct {
	threshold int           // consecutive failures before tripping
	cooldown  time.Duration // open → half-open delay
	now       func() time.Time

	mu          sync.Mutex
	state       State
	failures    int
	openedAt    time.Time
	halfOpenTry bool
}

type BreakerOption func(*Breaker)

func WithThreshold(n int) BreakerOption {
	return func(b *Breaker) { b.threshold = n }
}

func WithCooldown(d time.Duration) BreakerOption {
	return func(b *Breaker) { b.cooldown = d }
}

// NewBreaker constructs a breaker. Defaults: threshold 5, cooldown 30s.
func NewBreaker(opts ...BreakerOption) *Breaker {
	b := &Breaker{
		threshold: 5,
		cooldown:  30 * time.Second,
		now:       time.Now,
		state:     StateClosed,
	}
	for _, o := range opts {
		o(b)
	}
	return b
}

// Allow reports whether a request should be attempted. Call before sending.
func (b *Breaker) Allow() (allow bool, state State) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case StateClosed:
		return true, StateClosed
	case StateOpen:
		if b.now().Sub(b.openedAt) >= b.cooldown {
			b.state = StateHalfOpen
			b.halfOpenTry = true
			return true, StateHalfOpen
		}
		return false, StateOpen
	case StateHalfOpen:
		if b.halfOpenTry {
			b.halfOpenTry = false
			return true, StateHalfOpen
		}
		return false, StateHalfOpen
	}
	return true, b.state
}

// RecordSuccess closes the breaker.
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = StateClosed
	b.failures = 0
}

// RecordFailure increments the consecutive-failure counter and trips the
// breaker once it reaches the threshold.
func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == StateHalfOpen {
		b.state = StateOpen
		b.openedAt = b.now()
		b.halfOpenTry = false
		return
	}
	b.failures++
	if b.failures >= b.threshold {
		b.state = StateOpen
		b.openedAt = b.now()
	}
}

// State returns the current state.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// SetClock overrides the time source. Test-only.
func (b *Breaker) SetClock(now func() time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.now = now
}
