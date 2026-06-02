package httpclient_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/httpclient"
	"github.com/fromforgesoftware/go-kit/retry"
)

func TestClient_BaseURLJoin(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := httpclient.New(httpclient.WithBaseURL(srv.URL))
	req, _ := http.NewRequest(http.MethodGet, "/v1/widgets", nil)
	res, err := c.Do(context.Background(), req)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, "/v1/widgets", gotPath)
}

func TestClient_RetriesOn5xxThenSucceeds(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := httpclient.New(
		httpclient.WithBaseURL(srv.URL),
		httpclient.WithRetries(3),
		httpclient.WithBackoff(retry.WithConstantPolicy(1*time.Millisecond)),
	)
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	res, err := c.Do(context.Background(), req)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, int32(3), attempts.Load())
}

func TestClient_DoesNotRetryNonRetriableStatuses(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := httpclient.New(
		httpclient.WithBaseURL(srv.URL),
		httpclient.WithRetries(3),
		httpclient.WithBackoff(retry.WithConstantPolicy(1*time.Millisecond)),
	)
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	res, err := c.Do(context.Background(), req)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	assert.Equal(t, int32(1), attempts.Load(), "4xx must not retry")
}

func TestClient_ReplaysRequestBodyOnRetry(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen = append(seen, string(body))
		if len(seen) < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := httpclient.New(
		httpclient.WithBaseURL(srv.URL),
		httpclient.WithRetries(2),
		// POST is non-idempotent and isn't retried by default — opt
		// in explicitly to exercise the body-replay path.
		httpclient.WithRetryAll(),
		httpclient.WithBackoff(retry.WithConstantPolicy(1*time.Millisecond)),
	)
	req, _ := http.NewRequest(http.MethodPost, "/", strings.NewReader(`{"x":1}`))
	_, err := c.Do(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, []string{`{"x":1}`, `{"x":1}`}, seen)
}

func TestClient_HeaderFactoryAddsAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := httpclient.New(
		httpclient.WithBaseURL(srv.URL),
		httpclient.WithHeaders(func() http.Header { return http.Header{"Authorization": []string{"Bearer abc"}} }),
	)
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	res, err := c.Do(context.Background(), req)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, "Bearer abc", gotAuth)
}

func TestClient_BreakerOpensAfterRepeatedFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	b := httpclient.NewBreaker(httpclient.WithThreshold(2), httpclient.WithCooldown(time.Hour))
	c := httpclient.New(
		httpclient.WithBaseURL(srv.URL),
		httpclient.WithBreaker(b),
		httpclient.WithBackoff(retry.WithConstantPolicy(1*time.Millisecond)),
	)

	// 2 5xx responses → breaker trips
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		res, _ := c.Do(context.Background(), req)
		if res != nil {
			res.Body.Close()
		}
	}

	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	_, err := c.Do(context.Background(), req)
	assert.ErrorIs(t, err, httpclient.ErrBreakerOpen)
}

func TestClient_PerAttemptTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := httpclient.New(
		httpclient.WithBaseURL(srv.URL),
		httpclient.WithTimeout(10*time.Millisecond),
		httpclient.WithRetries(1),
	)
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	_, err := c.Do(context.Background(), req)
	assert.Error(t, err)
}
