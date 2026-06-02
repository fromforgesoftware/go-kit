package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Checker reports whether a subsystem is healthy. Implementations should
// respect ctx (which the aggregator gives a short timeout) and return a
// descriptive error on failure.
type Checker func(ctx context.Context) error

// Probes aggregates named liveness + readiness checks into HTTP handlers.
// Mount the handlers at /healthz (liveness) and /readyz (readiness) — a
// failed readiness check stops a load balancer from sending new traffic,
// a failed liveness check tells the orchestrator to restart the process.
//
//	probes := rest.NewProbes()
//	probes.AddReadiness("db", func(ctx context.Context) error { return db.PingContext(ctx) })
//	probes.AddLiveness("memory", checkers.MemoryHealthy)
//	router.Mount("/healthz", probes.LivenessHandler())
//	router.Mount("/readyz", probes.ReadinessHandler())
type Probes struct {
	mu             sync.RWMutex
	liveness       map[string]Checker
	readiness      map[string]Checker
	checkTimeout   time.Duration
	overallTimeout time.Duration
	manualReady    *Readiness // optional toggle ANDed with the readiness checks
}

type ProbeOption func(*Probes)

// WithProbeCheckTimeout sets the per-check deadline. Default: 1s.
func WithProbeCheckTimeout(d time.Duration) ProbeOption {
	return func(p *Probes) { p.checkTimeout = d }
}

// WithProbeOverallTimeout sets the total budget for a single probe request.
// Default: 3s.
func WithProbeOverallTimeout(d time.Duration) ProbeOption {
	return func(p *Probes) { p.overallTimeout = d }
}

// WithManualReadinessToggle wires in an existing Readiness toggle. When set,
// readiness reports "not ready" while the toggle is off, regardless of the
// registered checks. Use this for graceful shutdown draining.
func WithManualReadinessToggle(r *Readiness) ProbeOption {
	return func(p *Probes) { p.manualReady = r }
}

func NewProbes(opts ...ProbeOption) *Probes {
	p := &Probes{
		liveness:       make(map[string]Checker),
		readiness:      make(map[string]Checker),
		checkTimeout:   1 * time.Second,
		overallTimeout: 3 * time.Second,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func (p *Probes) AddLiveness(name string, c Checker) {
	p.mu.Lock()
	p.liveness[name] = c
	p.mu.Unlock()
}

func (p *Probes) AddReadiness(name string, c Checker) {
	p.mu.Lock()
	p.readiness[name] = c
	p.mu.Unlock()
}

// LivenessHandler runs registered liveness checks. 200 when all pass, 503 when
// any fail or time out.
func (p *Probes) LivenessHandler() http.HandlerFunc {
	return p.handler(func() map[string]Checker {
		p.mu.RLock()
		defer p.mu.RUnlock()
		return cloneChecks(p.liveness)
	}, nil)
}

// ReadinessHandler runs registered readiness checks. 200 when all pass, 503
// when any fail, time out, or the manual toggle is off.
func (p *Probes) ReadinessHandler() http.HandlerFunc {
	return p.handler(func() map[string]Checker {
		p.mu.RLock()
		defer p.mu.RUnlock()
		return cloneChecks(p.readiness)
	}, p.manualReady)
}

// LivenessEndpoint constructs the /healthz Endpoint for the REST server.
func (p *Probes) LivenessEndpoint() Endpoint {
	return NewEndpoint(http.MethodGet, "/healthz", p.LivenessHandler())
}

// ReadinessEndpoint constructs the /readyz Endpoint for the REST server.
func (p *Probes) ReadinessEndpoint() Endpoint {
	return NewEndpoint(http.MethodGet, "/readyz", p.ReadinessHandler())
}

type probeResult struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

func (p *Probes) handler(snapshot func() map[string]Checker, toggle *Readiness) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if toggle != nil && !toggle.IsReady() {
			writeProbe(w, http.StatusServiceUnavailable, probeResult{
				Status: "draining",
				Checks: map[string]string{"manual": "toggle is off"},
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), p.overallTimeout)
		defer cancel()

		checks := snapshot()
		results := runChecksInParallel(ctx, checks, p.checkTimeout)

		failed := false
		out := probeResult{Status: "ok", Checks: map[string]string{}}
		for name, err := range results {
			if err != nil {
				failed = true
				out.Checks[name] = err.Error()
			} else {
				out.Checks[name] = "ok"
			}
		}
		if failed {
			out.Status = "unhealthy"
			writeProbe(w, http.StatusServiceUnavailable, out)
			return
		}
		writeProbe(w, http.StatusOK, out)
	}
}

func cloneChecks(src map[string]Checker) map[string]Checker {
	dst := make(map[string]Checker, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func runChecksInParallel(parent context.Context, checks map[string]Checker, per time.Duration) map[string]error {
	results := make(map[string]error, len(checks))
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	for name, check := range checks {
		wg.Add(1)
		go func(name string, check Checker) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(parent, per)
			defer cancel()
			err := check(ctx)
			mu.Lock()
			results[name] = err
			mu.Unlock()
		}(name, check)
	}
	wg.Wait()
	return results
}

func writeProbe(w http.ResponseWriter, status int, body probeResult) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
