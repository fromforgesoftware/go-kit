package rest

import (
	"net/http"
	"sync/atomic"
)

// Readiness is a thread-safe ready/not-ready toggle backed by an
// HTTP handler. The fx module installs one as /readyz, registers it for
// /readyz, starts it as ready, then flips it to not-ready at the top of
// the shutdown hook so load balancers stop sending new traffic before
// the in-flight requests finish draining.
type Readiness struct {
	ready atomic.Bool
}

// NewReadiness returns a Readiness initialised to ready=true.
func NewReadiness() *Readiness {
	r := &Readiness{}
	r.ready.Store(true)
	return r
}

// SetReady flips the readiness state.
func (r *Readiness) SetReady(ready bool) { r.ready.Store(ready) }

// IsReady reports the current state.
func (r *Readiness) IsReady() bool { return r.ready.Load() }

// Handler returns the HTTP handler — 200 when ready, 503 when not.
func (r *Readiness) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if r.ready.Load() {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

// ReadinessEndpoint constructs the /readyz Endpoint to register on the
// REST server.
func ReadinessEndpoint(r *Readiness) Endpoint {
	return NewEndpoint(http.MethodGet, "/readyz", r.Handler())
}
