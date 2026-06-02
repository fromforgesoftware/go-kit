package openapi

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/fromforgesoftware/go-kit/openapi/internal/swaggestimpl"
)

// Collector orchestrates OpenAPI spec generation across the kit. It
// owns a Reflector, holds per-route annotation registrations, and
// serves the final spec JSON.
//
// Lifetime: one Collector per server. Constructed by rest.WithOpenAPI;
// fed by router.build as routes are registered; queried by the spec
// endpoint at request time.
//
// All public methods are safe for concurrent use.
type Collector struct {
	mu          sync.Mutex
	ref         Reflector
	cfg         SpecConfig
	annotations map[string][]func(oc OperationContext) error
	skipPaths   map[string]struct{}
}

// NewCollector returns a Collector backed by the given Reflector and
// pre-configured with the spec-level options in cfg. Applies defaults
// (SpecPath, UIPath, …) before propagating into the spec.
func NewCollector(ref Reflector, cfg SpecConfig) (*Collector, error) {
	cfg.ApplyDefaults()
	if err := ref.ApplySpecConfig(cfg); err != nil {
		return nil, fmt.Errorf("openapi: apply spec config: %w", err)
	}
	return &Collector{
		ref:         ref,
		cfg:         cfg,
		annotations: map[string][]func(oc OperationContext) error{},
		skipPaths:   map[string]struct{}{},
	}, nil
}

// SkipPath marks a path that should not be collected into the spec.
// The kit uses this to hide infrastructure routes (the spec endpoint
// itself, the UI mount, health probes) — they're not part of the
// service's API surface.
func (c *Collector) SkipPath(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.skipPaths[path] = struct{}{}
}

// SpecConfig returns the resolved config, including any defaults
// filled by ApplyDefaults. Used by rest.WithOpenAPI to know which
// paths to mount the spec + UI at.
func (c *Collector) SpecConfig() SpecConfig {
	return c.cfg
}

// AnnotateOperation registers a setup function for the (method,
// pattern) operation. Setup runs during CollectOperation, after the
// handler's own Preparer setup and before the caller-supplied
// annotations. Useful for cross-cutting concerns (e.g. applying
// DefaultSecurity).
func (c *Collector) AnnotateOperation(method, pattern string, fn func(oc OperationContext) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.annotations[opKey(method, pattern)] = append(c.annotations[opKey(method, pattern)], fn)
}

// CollectOperation runs the full setup chain for one operation and
// registers it on the spec:
//
//  1. Pre-registered annotations from AnnotateOperation.
//  2. The handler's Preparer (if the http.Handler implements
//     openapi.Preparer).
//  3. The caller-supplied annotations passed in this call.
//
// Step 4 (after all setups) applies DefaultSecurity if the operation
// hasn't already set its own security.
func (c *Collector) CollectOperation(
	method, pattern string,
	handler http.Handler,
	annotations ...func(oc OperationContext) error,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, skip := c.skipPaths[pattern]; skip {
		return nil
	}

	oc, err := c.ref.NewOperationContext(method, pattern)
	if err != nil {
		return fmt.Errorf("openapi: new operation context for %s %s: %w", method, pattern, err)
	}

	for _, fn := range c.annotations[opKey(method, pattern)] {
		if err := fn(oc); err != nil {
			return fmt.Errorf("openapi: annotation for %s %s: %w", method, pattern, err)
		}
	}

	if p, ok := handler.(Preparer); ok {
		if err := p.SetupOperation(oc); err != nil {
			return fmt.Errorf("openapi: handler setup for %s %s: %w", method, pattern, err)
		}
	}

	for _, fn := range annotations {
		if err := fn(oc); err != nil {
			return fmt.Errorf("openapi: extra annotation for %s %s: %w", method, pattern, err)
		}
	}

	// DefaultSecurity is applied at the spec root via ApplySpecConfig,
	// so we don't need to set it per-operation. Operations override
	// the root default by calling SetSecurity themselves.

	// Auto-inject path parameters from the URL pattern so handlers
	// that consume r.PathValue("id") at runtime (the kit's
	// convention) don't have to declare them via struct tags. Without
	// this swaggest fails the "undefined path parameter" validator.
	if ocImpl, ok := oc.(*operationContext); ok {
		for _, name := range extractPathParams(pattern) {
			ocImpl.ensurePathParameter(name)
		}
	}

	if err := c.ref.AddOperation(oc); err != nil {
		return fmt.Errorf("openapi: add operation %s %s: %w", method, pattern, err)
	}

	// Examples are buffered on the operationContext during setup and
	// applied AFTER AddOperation has populated the underlying
	// Operation's RequestBody / Responses content maps.
	if ocImpl, ok := oc.(*operationContext); ok {
		if reflImpl, ok := c.ref.(*reflector); ok {
			if op := swaggestimpl.OperationOn(reflImpl.r, method, pattern); op != nil {
				reqEx, respEx := ocImpl.pendingExamples()
				for _, ex := range reqEx {
					swaggestimpl.AddRequestExample(op, ex.name, ex.value)
				}
				for _, ex := range respEx {
					swaggestimpl.AddResponseExample(op, ex.status, ex.name, ex.value)
				}
			}
		}
	}
	return nil
}

// ServeHTTP serves the assembled spec as application/json.
func (c *Collector) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()

	body, err := c.ref.SpecJSON()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(body)
}

func opKey(method, pattern string) string {
	return method + " " + pattern
}

// extractPathParams pulls every {name} (or {name...}) segment out of
// an OpenAPI-style path pattern, returning the bare names without
// braces. Conservative — anything weird (nested braces, empty names)
// is ignored.
func extractPathParams(pattern string) []string {
	var out []string
	for {
		open := strings.IndexByte(pattern, '{')
		if open < 0 {
			return out
		}
		close := strings.IndexByte(pattern[open:], '}')
		if close < 0 {
			return out
		}
		close += open
		name := pattern[open+1 : close]
		// stdlib mux supports {key...} wildcards — strip the suffix.
		name = strings.TrimSuffix(name, "...")
		if name != "" {
			out = append(out, name)
		}
		pattern = pattern[close+1:]
	}
}
