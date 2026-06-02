package ratelimit_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/ratelimit"
)

func TestTokenBucket_BurstThenDeny(t *testing.T) {
	s := ratelimit.NewInMemoryStore()
	p := ratelimit.Policy{Limit: 10, Window: time.Second, Burst: 5}
	now := time.Unix(100, 0)

	for i := 0; i < 5; i++ {
		r, err := s.Take(context.Background(), "k", p, 1, now)
		require.NoError(t, err)
		assert.True(t, r.Allowed, "take %d should pass within burst", i)
	}
	r, err := s.Take(context.Background(), "k", p, 1, now)
	require.NoError(t, err)
	assert.False(t, r.Allowed, "6th exceeds burst of 5")
	assert.Equal(t, 0, r.Remaining)
	assert.Greater(t, r.RetryAfter, time.Duration(0))
}

func TestTokenBucket_Refills(t *testing.T) {
	s := ratelimit.NewInMemoryStore()
	p := ratelimit.Policy{Limit: 10, Window: time.Second} // burst defaults to 10
	now := time.Unix(100, 0)
	for i := 0; i < 10; i++ {
		_, _ = s.Take(context.Background(), "k", p, 1, now)
	}
	r, _ := s.Take(context.Background(), "k", p, 1, now)
	require.False(t, r.Allowed)

	// 0.5s later → +5 tokens at 10/s.
	r, _ = s.Take(context.Background(), "k", p, 1, now.Add(500*time.Millisecond))
	assert.True(t, r.Allowed)
	assert.Equal(t, 4, r.Remaining)
}

func TestAllowN_Cost(t *testing.T) {
	s := ratelimit.NewInMemoryStore()
	p := ratelimit.Policy{Limit: 10, Window: time.Second, Burst: 10}
	now := time.Unix(100, 0)

	r, _ := s.Take(context.Background(), "k", p, 7, now)
	require.True(t, r.Allowed)
	assert.Equal(t, 3, r.Remaining)

	r, _ = s.Take(context.Background(), "k", p, 5, now)
	assert.False(t, r.Allowed, "only 3 tokens left, cost 5 denied")
}

// TestConcurrency_AdmitsExactlyCapacity is the mutex-correctness check: with a
// window long enough that no refill happens during the test, N concurrent
// takes admit exactly Burst — never more (no double-spend).
func TestConcurrency_AdmitsExactlyCapacity(t *testing.T) {
	s := ratelimit.NewInMemoryStore()
	p := ratelimit.Policy{Limit: 100, Window: time.Hour, Burst: 50}
	now := time.Unix(100, 0)

	var allowed int64
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r, _ := s.Take(context.Background(), "k", p, 1, now); r.Allowed {
				atomic.AddInt64(&allowed, 1)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(50), allowed)
}

func TestHTTPMiddleware_429(t *testing.T) {
	lim := ratelimit.New(ratelimit.NewInMemoryStore())
	mw := ratelimit.NewHTTPMiddleware(lim, ratelimit.ByIP(),
		ratelimit.StaticPolicy(ratelimit.Policy{Limit: 2, Window: time.Minute, Burst: 2}))
	h := mw.Intercept(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	call := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "192.0.2.7:5555"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	assert.Equal(t, http.StatusOK, call().Code)
	assert.Equal(t, http.StatusOK, call().Code)
	third := call()
	assert.Equal(t, http.StatusTooManyRequests, third.Code)
	assert.Equal(t, "2", third.Header().Get("RateLimit-Limit"))
	assert.NotEmpty(t, third.Header().Get("Retry-After"))
}
