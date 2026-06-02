package rest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fromforgesoftware/go-kit/transport/rest"
)

func TestCORSDisallowedOriginDoesNotEchoAllowedOrigin(t *testing.T) {
	mw := rest.NewCORSMiddlewareWithOrigins([]string{"https://allowed.example"})
	handler := mw.Intercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"),
		"disallowed origin must NOT receive the allowed-origin header")
}

func TestCORSAllowedOriginEchoesOnlyRequestOrigin(t *testing.T) {
	mw := rest.NewCORSMiddlewareWithOrigins([]string{"https://a.example", "https://b.example"})
	handler := mw.Intercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://a.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "https://a.example", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSWildcardAcceptsAny(t *testing.T) {
	mw := rest.NewCORSMiddleware()
	handler := mw.Intercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://anything.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "https://anything.example", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSAlwaysSetsVaryOrigin(t *testing.T) {
	mw := rest.NewCORSMiddleware()
	handler := mw.Intercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	assert.Contains(t, rec.Header().Values("Vary"), "Origin")
}

func TestCORSPreflightReturns204(t *testing.T) {
	mw := rest.NewCORSMiddlewareWithOrigins([]string{"https://a.example"})
	handler := mw.Intercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run for OPTIONS preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	req.Header.Set("Origin", "https://a.example")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}
