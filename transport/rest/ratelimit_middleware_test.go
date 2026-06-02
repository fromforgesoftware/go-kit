package rest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/transport/rest"
)

func TestRateLimitMiddleware_AllowsBurstThenRejects(t *testing.T) {
	called := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	})

	m := rest.NewRateLimitMiddleware(1, 3) // 1 RPS, burst 3
	srv := httptest.NewServer(m.Intercept(handler))
	defer srv.Close()

	for i := 0; i < 3; i++ {
		res, err := http.Get(srv.URL)
		require.NoError(t, err)
		require.NoError(t, res.Body.Close())
		assert.Equal(t, http.StatusOK, res.StatusCode, "burst slot %d should pass", i)
	}

	res, err := http.Get(srv.URL)
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())
	assert.Equal(t, http.StatusTooManyRequests, res.StatusCode, "4th call within the same second should be limited")
	assert.NotEmpty(t, res.Header.Get("Retry-After"))
	assert.Equal(t, 3, called)
}

func TestRateLimitMiddleware_PerKeyIsolated(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	m := rest.NewRateLimitMiddleware(1, 1, rest.WithRateLimitKeyFunc(func(r *http.Request) string {
		return r.Header.Get("X-User")
	}))
	srv := httptest.NewServer(m.Intercept(handler))
	defer srv.Close()

	req := func(user string) int {
		r, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
		r.Header.Set("X-User", user)
		res, err := http.DefaultClient.Do(r)
		require.NoError(t, err)
		require.NoError(t, res.Body.Close())
		return res.StatusCode
	}

	assert.Equal(t, http.StatusOK, req("alice"))
	assert.Equal(t, http.StatusTooManyRequests, req("alice"))
	assert.Equal(t, http.StatusOK, req("bob"))
}
