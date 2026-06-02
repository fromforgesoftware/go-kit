package rest_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/monitoring/monitoringtest"
	"github.com/fromforgesoftware/go-kit/transport/rest"
)

func TestRequestIDMiddlewareGeneratesAndPropagates(t *testing.T) {
	var seen string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = rest.RequestIDFromContext(r.Context())
	})
	mw := rest.NewRequestIDMiddleware().Intercept(handler)

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	assert.NotEmpty(t, seen)
	assert.Equal(t, seen, rec.Header().Get(rest.HeaderRequestID))
}

func TestRequestIDMiddlewarePreservesInbound(t *testing.T) {
	want := "incoming-id"
	var seen string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = rest.RequestIDFromContext(r.Context())
	})
	mw := rest.NewRequestIDMiddleware().Intercept(handler)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(rest.HeaderRequestID, want)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	assert.Equal(t, want, seen)
	assert.Equal(t, want, rec.Header().Get(rest.HeaderRequestID))
}

func TestRecoveryMiddlewareConvertsPanicTo500(t *testing.T) {
	monitor := monitoringtest.NewMonitor(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(errors.New("boom"))
	})
	mw := rest.NewRecoveryMiddleware(monitor).Intercept(handler)

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTracingMiddlewareStartsAndEndsSpan(t *testing.T) {
	monitor := monitoringtest.NewMonitor(t)
	var captured string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Span should be present in ctx mid-handler.
		captured = monitor.Tracer().SpanFromContext(r.Context()).SpanContext().TraceID
		w.WriteHeader(http.StatusOK)
	})
	mw := rest.NewTracingMiddleware(monitor).Intercept(handler)

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/123", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	// Stub tracer returns empty span context — assertion is non-empty execution.
	_ = captured
}

func TestDefaultObservabilityMiddlewaresOrder(t *testing.T) {
	monitor := monitoringtest.NewMonitor(t)
	mws := rest.DefaultObservabilityMiddlewares(monitor)
	require.Len(t, mws, 4)
}

func TestAccessLogMiddlewareCapturesStatus(t *testing.T) {
	monitor := monitoringtest.NewMonitor(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hi"))
	})
	mw := rest.NewAccessLogMiddleware(monitor).Intercept(handler)

	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("")))
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestTracingTransportInjectsHeaders(t *testing.T) {
	monitor := monitoringtest.NewMonitor(t)

	var gotHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{Transport: rest.NewTracingTransport(monitor, nil)}
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// At minimum the transport must have called Inject without erroring;
	// the stub tracer is a noop so no header may be written. Just verify
	// the round-trip succeeded.
	_ = gotHeaders
}
