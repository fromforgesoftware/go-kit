package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/transport/rest"
)

func okCheck(context.Context) error { return nil }
func failCheck(context.Context) error {
	return errors.New("backend offline")
}

func TestProbes_LivenessAllOk(t *testing.T) {
	p := rest.NewProbes()
	p.AddLiveness("memory", okCheck)
	p.AddLiveness("disk", okCheck)

	rr := httptest.NewRecorder()
	p.LivenessHandler()(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestProbes_ReadinessFailsWhenAnyCheckFails(t *testing.T) {
	p := rest.NewProbes()
	p.AddReadiness("db", failCheck)
	p.AddReadiness("cache", okCheck)

	rr := httptest.NewRecorder()
	p.ReadinessHandler()(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "unhealthy", body["status"])
	checks := body["checks"].(map[string]any)
	assert.Equal(t, "backend offline", checks["db"])
	assert.Equal(t, "ok", checks["cache"])
}

func TestProbes_ManualReadinessToggleStopsServingEvenIfChecksPass(t *testing.T) {
	toggle := rest.NewReadiness()
	p := rest.NewProbes(rest.WithManualReadinessToggle(toggle))
	p.AddReadiness("db", okCheck)

	toggle.SetReady(false)
	rr := httptest.NewRecorder()
	p.ReadinessHandler()(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "draining", body["status"])
}

func TestProbes_RespectsPerCheckTimeout(t *testing.T) {
	slow := func(ctx context.Context) error {
		select {
		case <-time.After(200 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	p := rest.NewProbes(rest.WithProbeCheckTimeout(20*time.Millisecond), rest.WithProbeOverallTimeout(time.Second))
	p.AddReadiness("slow", slow)

	rr := httptest.NewRecorder()
	p.ReadinessHandler()(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestProbes_EmptyChecksReturnOk(t *testing.T) {
	p := rest.NewProbes()

	rr := httptest.NewRecorder()
	p.LivenessHandler()(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestProbes_EndpointsRegisterCorrectPaths(t *testing.T) {
	p := rest.NewProbes()
	live := p.LivenessEndpoint()
	ready := p.ReadinessEndpoint()
	assert.Equal(t, "/healthz", live.Path())
	assert.Equal(t, "/readyz", ready.Path())
	assert.Equal(t, http.MethodGet, live.Method())
}
