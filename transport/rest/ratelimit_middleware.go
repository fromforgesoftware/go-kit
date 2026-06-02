package rest

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// KeyFunc derives the rate-limit bucket key from a request. Defaults to
// the client IP (X-Forwarded-For first, then RemoteAddr).
type KeyFunc func(*http.Request) string

func defaultKeyFunc(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		for i, c := range fwd {
			if c == ',' {
				return fwd[:i]
			}
		}
		return fwd
	}
	return r.RemoteAddr
}

// RateLimitMiddleware caps requests per key using a token-bucket per bucket.
// Buckets are GC'd after idle to keep the map bounded.
type RateLimitMiddleware struct {
	rate    float64 // tokens per second
	burst   int     // bucket capacity
	keyFunc KeyFunc
	idleTTL time.Duration

	mu      sync.Mutex
	buckets map[string]*tokenBucket
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

type rateLimitOption func(*RateLimitMiddleware)

func WithRateLimitKeyFunc(fn KeyFunc) rateLimitOption {
	return func(m *RateLimitMiddleware) { m.keyFunc = fn }
}

func WithRateLimitIdleTTL(d time.Duration) rateLimitOption {
	return func(m *RateLimitMiddleware) { m.idleTTL = d }
}

// NewRateLimitMiddleware builds a per-key token bucket. `rps` is the steady
// refill rate; `burst` is the max queued tokens. Defaults: key by IP, GC
// idle buckets after 10 minutes.
func NewRateLimitMiddleware(rps float64, burst int, opts ...rateLimitOption) *RateLimitMiddleware {
	m := &RateLimitMiddleware{
		rate:    rps,
		burst:   burst,
		keyFunc: defaultKeyFunc,
		idleTTL: 10 * time.Minute,
		buckets: make(map[string]*tokenBucket),
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

func (m *RateLimitMiddleware) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := m.keyFunc(r)
		allowed, retryAfter := m.consume(key)
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds()+1)))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *RateLimitMiddleware) consume(key string) (allowed bool, retryAfter time.Duration) {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	m.gcLocked(now)

	b, ok := m.buckets[key]
	if !ok {
		b = &tokenBucket{tokens: float64(m.burst), last: now}
		m.buckets[key] = b
	}

	elapsed := now.Sub(b.last).Seconds()
	b.tokens = minF(float64(m.burst), b.tokens+elapsed*m.rate)
	b.last = now

	if b.tokens < 1 {
		needed := 1 - b.tokens
		return false, time.Duration(needed/m.rate*float64(time.Second)) + time.Millisecond
	}
	b.tokens--
	return true, 0
}

func (m *RateLimitMiddleware) gcLocked(now time.Time) {
	for k, b := range m.buckets {
		if now.Sub(b.last) > m.idleTTL {
			delete(m.buckets, k)
		}
	}
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
