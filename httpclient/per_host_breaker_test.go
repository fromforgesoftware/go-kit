package httpclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/httpclient"
)

func TestPerHostBreaker_LazilyCreatesPerHost(t *testing.T) {
	p := httpclient.NewPerHostBreaker(3, time.Second)
	a := p.For("a.example.com")
	b := p.For("b.example.com")
	a2 := p.For("a.example.com")
	assert.True(t, a == a2, "same host returns the same breaker")
	assert.False(t, a == b, "different hosts get different breakers")
}

func TestPerHostBreaker_IsolatesFailures(t *testing.T) {
	// One server returns 503 (will trip its breaker), the other 200.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer good.Close()

	c := httpclient.New(
		httpclient.WithBreakerPerHost(2, time.Minute),
	)

	// Two failed calls to bad → its breaker opens.
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest(http.MethodGet, bad.URL, nil)
		_, _ = c.Do(context.Background(), req)
	}

	// Bad now fails fast.
	req, _ := http.NewRequest(http.MethodGet, bad.URL, nil)
	_, err := c.Do(context.Background(), req)
	require.ErrorIs(t, err, httpclient.ErrBreakerOpen, "bad host's breaker should be open")

	// Good remains unaffected — its per-host breaker has zero failures.
	req, _ = http.NewRequest(http.MethodGet, good.URL, nil)
	res, err := c.Do(context.Background(), req)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)
}

func TestClient_DoesNotRetryNonIdempotentByDefault(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := httpclient.New(
		httpclient.WithBaseURL(srv.URL),
		httpclient.WithRetries(5),
	)
	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	res, err := c.Do(context.Background(), req)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, int32(1), attempts.Load(), "POST must not retry by default")
}

func TestClient_WithRetryAllRetriesNonIdempotent(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := httpclient.New(
		httpclient.WithBaseURL(srv.URL),
		httpclient.WithRetries(3),
		httpclient.WithRetryAll(),
	)
	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	res, err := c.Do(context.Background(), req)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, int32(3), attempts.Load(), "WithRetryAll should retry POST")
}
