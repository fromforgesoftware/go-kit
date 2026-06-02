package idempotency_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/idempotency"
)

func mkServer(t *testing.T, store idempotency.Store, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mw := idempotency.RESTMiddleware(idempotency.Config{
		Store:       store,
		DefaultTTL:  time.Minute,
		WaitTimeout: 2 * time.Second,
	})
	srv := httptest.NewServer(mw(handler))
	t.Cleanup(srv.Close)
	return srv
}

func TestPassThroughWithoutHeader(t *testing.T) {
	var hits atomic.Int32
	srv := mkServer(t, idempotency.NewMemoryStore(idempotency.MemoryConfig{}), func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("ok"))
	})

	for i := 0; i < 3; i++ {
		resp, err := http.Post(srv.URL, "application/json", strings.NewReader("{}"))
		require.NoError(t, err)
		_ = resp.Body.Close()
	}
	assert.Equal(t, int32(3), hits.Load(), "no header → handler runs every time")
}

func TestReplayServesCachedResponse(t *testing.T) {
	var hits atomic.Int32
	store := idempotency.NewMemoryStore(idempotency.MemoryConfig{})
	srv := mkServer(t, store, func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":42}`))
	})

	makeReq := func() *http.Response {
		req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"amt":10}`))
		req.Header.Set("Idempotency-Key", "k-1")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		return resp
	}

	r1 := makeReq()
	body1, _ := io.ReadAll(r1.Body)
	_ = r1.Body.Close()
	assert.Equal(t, http.StatusCreated, r1.StatusCode)
	assert.JSONEq(t, `{"id":42}`, string(body1))

	r2 := makeReq()
	body2, _ := io.ReadAll(r2.Body)
	_ = r2.Body.Close()
	assert.Equal(t, http.StatusCreated, r2.StatusCode)
	assert.JSONEq(t, `{"id":42}`, string(body2))
	assert.Equal(t, "true", r2.Header.Get("Idempotency-Replay"))
	assert.Equal(t, int32(1), hits.Load(), "handler runs once; replay served from store")
}

func TestHashMismatchRejects(t *testing.T) {
	srv := mkServer(t, idempotency.NewMemoryStore(idempotency.MemoryConfig{}), func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"amt":10}`))
	req.Header.Set("Idempotency-Key", "k-1")
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()

	req2, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"amt":99}`))
	req2.Header.Set("Idempotency-Key", "k-1")
	resp2, _ := http.DefaultClient.Do(req2)
	defer func() { _ = resp2.Body.Close() }()
	assert.Equal(t, http.StatusConflict, resp2.StatusCode)
}

func TestConcurrentSameKeyCollapsesToOne(t *testing.T) {
	var hits atomic.Int32
	store := idempotency.NewMemoryStore(idempotency.MemoryConfig{})
	srv := mkServer(t, store, func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		time.Sleep(80 * time.Millisecond) // simulate slow handler
		_, _ = w.Write([]byte(`{"once":1}`))
	})

	var wg sync.WaitGroup
	bodies := make([]string, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{"a":1}`))
			req.Header.Set("Idempotency-Key", "race-1")
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			bodies[i] = string(body)
		}(i)
	}
	wg.Wait()
	assert.Equal(t, int32(1), hits.Load(), "handler must run exactly once across concurrent same-key requests")
	for _, b := range bodies {
		assert.JSONEq(t, `{"once":1}`, b)
	}
}

func TestServerErrorNotCached(t *testing.T) {
	var hits atomic.Int32
	store := idempotency.NewMemoryStore(idempotency.MemoryConfig{})
	srv := mkServer(t, store, func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})

	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{}`))
		req.Header.Set("Idempotency-Key", "err-1")
		resp, _ := http.DefaultClient.Do(req)
		_ = resp.Body.Close()
	}
	assert.Equal(t, int32(2), hits.Load(), "5xx must not be cached; subsequent retries re-run the handler")
}

func TestClientErrorIsCached(t *testing.T) {
	var hits atomic.Int32
	store := idempotency.NewMemoryStore(idempotency.MemoryConfig{})
	srv := mkServer(t, store, func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad"))
	})

	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(`{}`))
		req.Header.Set("Idempotency-Key", "cli-1")
		resp, _ := http.DefaultClient.Do(req)
		_ = resp.Body.Close()
	}
	assert.Equal(t, int32(1), hits.Load(), "deliberate 4xx is cached")
}

func TestPanicsOnMissingStore(t *testing.T) {
	assert.Panics(t, func() {
		idempotency.RESTMiddleware(idempotency.Config{})
	})
}
