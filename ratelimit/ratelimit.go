package ratelimit

import (
	"context"
	"time"
)

// Policy is a token-bucket rate: Limit tokens refilled over Window, with a
// bucket capacity of Burst (defaults to Limit when zero).
type Policy struct {
	Limit  int
	Window time.Duration
	Burst  int
}

// Capacity is the bucket size (Burst, or Limit when Burst is unset).
func (p Policy) Capacity() float64 {
	if p.Burst > 0 {
		return float64(p.Burst)
	}
	return float64(p.Limit)
}

// RefillPerSecond is how fast tokens are replenished.
func (p Policy) RefillPerSecond() float64 {
	if p.Window <= 0 || p.Limit <= 0 {
		return 0
	}
	return float64(p.Limit) / p.Window.Seconds()
}

// Result is the outcome of a limit check.
type Result struct {
	Allowed    bool
	Limit      int           // Policy.Limit, for the RateLimit-Limit header
	Remaining  int           // whole tokens left after this call
	ResetAfter time.Duration // until the bucket refills to full
	RetryAfter time.Duration // until cost tokens are available again (deny only)
}

// Store holds bucket state and performs the atomic take. Implementations:
// in-memory (mutex) now; Redis (Lua) later for multi-replica correctness.
type Store interface {
	Take(ctx context.Context, key string, policy Policy, cost int, now time.Time) (Result, error)
}

// Limiter checks a keyed caller against a Policy.
type Limiter interface {
	// Allow consumes one token for key under policy.
	Allow(ctx context.Context, key string, policy Policy) (Result, error)
	// AllowN consumes cost tokens (e.g. an expensive endpoint costs more).
	AllowN(ctx context.Context, key string, policy Policy, cost int) (Result, error)
}
