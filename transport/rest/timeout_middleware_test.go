package rest_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/go-kit/transport/rest"
)

func TestTimeoutMiddleware_RespondsWith503OnSlowHandler(t *testing.T) {
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		w.WriteHeader(http.StatusOK)
	})

	m := rest.NewTimeoutMiddleware(50*time.Millisecond, rest.WithTimeoutMessage("slow"))
	srv := httptest.NewServer(m.Intercept(slow))
	defer srv.Close()

	res, err := http.Get(srv.URL)
	assert.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, res.StatusCode)
}

func TestTimeoutMiddleware_PassThroughFastHandler(t *testing.T) {
	fast := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	m := rest.NewTimeoutMiddleware(200 * time.Millisecond)
	srv := httptest.NewServer(m.Intercept(fast))
	defer srv.Close()

	res, err := http.Get(srv.URL)
	assert.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)
}

func TestContextTimeoutMiddleware_CancelsTheRequestContext(t *testing.T) {
	var cancelObserved atomic.Bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			cancelObserved.Store(true)
		case <-time.After(200 * time.Millisecond):
		}
		w.WriteHeader(http.StatusOK)
	})

	m := rest.NewContextTimeoutMiddleware(20 * time.Millisecond)
	srv := httptest.NewServer(m.Intercept(handler))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	res, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer res.Body.Close()
	assert.True(t, cancelObserved.Load(), "handler must observe ctx cancellation")
}
