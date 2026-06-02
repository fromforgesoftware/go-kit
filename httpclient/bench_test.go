package httpclient_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fromforgesoftware/go-kit/httpclient"
)

// BenchmarkClientDo_HappyPath measures the per-request overhead of
// the kit's outbound client: retry-eligibility check, breaker
// consult, header injection, body buffering, retry-wrap. The fixture
// server returns 200 immediately so we isolate the kit overhead from
// network / handler cost.
func BenchmarkClientDo_HappyPath(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := httpclient.New(httpclient.WithBaseURL(srv.URL))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		res, err := c.Do(context.Background(), req)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}
}

// BenchmarkClientDo_WithPerHostBreaker measures the additional cost
// of the lazy per-host breaker map lookup vs the base client.
func BenchmarkClientDo_WithPerHostBreaker(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := httpclient.New(
		httpclient.WithBaseURL(srv.URL),
		httpclient.WithBreakerPerHost(10, 0),
	)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(http.MethodGet, "/", nil)
		res, err := c.Do(context.Background(), req)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}
}
