package rest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/transport/rest"
)

func TestReadinessHandlerInitiallyReady(t *testing.T) {
	r := rest.NewReadiness()
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestReadinessHandlerFlipsToNotReady(t *testing.T) {
	r := rest.NewReadiness()
	r.SetReady(false)
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.False(t, r.IsReady())
}

func TestReadinessEndpointPathAndMethod(t *testing.T) {
	r := rest.NewReadiness()
	ep := rest.ReadinessEndpoint(r)
	require.NotNil(t, ep)
	assert.Equal(t, http.MethodGet, ep.Method())
	assert.Equal(t, "/readyz", ep.Path())
}

func TestNewDefaultHTTPClientReturnsTunedTransport(t *testing.T) {
	c := rest.NewDefaultHTTPClient()
	require.NotNil(t, c)
	require.NotNil(t, c.Transport)
	tr, ok := c.Transport.(*http.Transport)
	require.True(t, ok, "default transport should be *http.Transport for callers to introspect")
	assert.Greater(t, tr.MaxIdleConnsPerHost, 2, "should not use the stdlib default of 2 idle conns/host")
}
