package httpclient

import (
	"sync"
	"time"
)

// PerHostBreaker keeps a separate *Breaker per upstream host so one
// flaky downstream doesn't poison unrelated calls. Breakers are
// created lazily on first request to a given host.
type PerHostBreaker struct {
	threshold int
	cooldown  time.Duration

	mu       sync.RWMutex
	breakers map[string]*Breaker
}

// NewPerHostBreaker returns a manager that builds per-host breakers
// using the supplied threshold + cooldown.
func NewPerHostBreaker(threshold int, cooldown time.Duration) *PerHostBreaker {
	return &PerHostBreaker{
		threshold: threshold,
		cooldown:  cooldown,
		breakers:  map[string]*Breaker{},
	}
}

// For returns the breaker for host, creating one on demand.
func (p *PerHostBreaker) For(host string) *Breaker {
	p.mu.RLock()
	b, ok := p.breakers[host]
	p.mu.RUnlock()
	if ok {
		return b
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if b, ok = p.breakers[host]; ok {
		return b
	}
	b = NewBreaker(WithThreshold(p.threshold), WithCooldown(p.cooldown))
	p.breakers[host] = b
	return b
}

// Snapshot returns the current per-host breaker state. Useful for
// /healthz handlers that want to expose breaker status.
func (p *PerHostBreaker) Snapshot() map[string]State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[string]State, len(p.breakers))
	for h, b := range p.breakers {
		_, s := b.Allow()
		out[h] = s
	}
	return out
}
