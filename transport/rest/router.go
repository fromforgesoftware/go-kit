package rest

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/fromforgesoftware/go-kit/openapi"
)

// Router is the registration surface for HTTP routes. A Controller
// receives a Router and declares endpoints via Get/Post/Put/Patch/
// Delete/Method, grouping them by URL prefix with Route(), stacking
// middleware with Use() (cumulative for the scope) or With() (one-off
// for a single registration chain).
//
// The Router collects registrations during the boot phase; at server
// build time the tree is flattened into a single http.ServeMux. There
// is no runtime routing layer beyond what stdlib provides — the
// Router is just an accumulator that knows how to compose path
// prefixes and middleware stacks before handing off to mux.Handle.
//
// Path conventions:
//
//   - prefix on Route() and path on Get/Post/... are joined naively
//     with "/"; leading/trailing slashes are normalised so callers
//     don't have to think about it
//   - stdlib mux path params ({id}, {key...}) flow through unchanged
//   - the empty path "" on a leaf method (e.g. r.Get("", h)) means
//     "the bare prefix" — handy inside a Route("/{id}", ...) block
//
// Middleware ordering: the first middleware in a Use(a, b, c) call
// is the OUTERMOST (runs first, returns last). Chained scopes
// concatenate root-to-leaf: a route registered inside
// Route("/a", func(r){ r.Use(mwA); r.Route("/b", func(r){ r.Use(mwB); r.Get("/c", h) }) })
// is wrapped as mwA(mwB(h)) — outer scope's middleware is outermost.
type Router interface {
	// Route mounts a sub-router at prefix. The fn receives the
	// sub-router and registers its own routes / nested groups /
	// middleware against it; nothing leaks out to the parent except
	// the registered routes themselves.
	Route(prefix string, fn func(Router))

	// With returns a sub-router whose subsequent registrations are
	// wrapped with the given middleware, scoped just to the returned
	// router. Doesn't mutate the parent. Idiom: r.With(authMW).Post(...).
	With(mw ...Middleware) Router

	// Use adds middleware cumulatively to the current router's scope.
	// All subsequent registrations on this router (and any nested
	// Route()) inherit it.
	Use(mw ...Middleware)

	// Per-method handler registration. Path is joined onto the
	// current scope's prefix.
	Get(path string, h http.Handler)
	Post(path string, h http.Handler)
	Put(path string, h http.Handler)
	Patch(path string, h http.Handler)
	Delete(path string, h http.Handler)

	// Method is the generic registration helper. Use for non-standard
	// verbs (HEAD, OPTIONS) when you really need them.
	Method(method, path string, h http.Handler)

	// Mount attaches an arbitrary http.Handler at prefix. The handler
	// receives requests with the prefix stripped (via
	// http.StripPrefix). Useful for sub-services, file servers, or a
	// third-party router we want to delegate to.
	Mount(prefix string, h http.Handler)
}

// NewRouter returns a fresh top-level Router. Callers usually don't
// construct one directly — NewServer builds the root Router, passes
// it to each Controller's Routes() method, and flattens the result
// into the mux.
func NewRouter() Router {
	return &router{}
}

// BuildHandler is a convenience for tests and embedded uses that want
// the same routing semantics as NewServer without the http.Server
// wrapper: it runs each Controller's Routes() against a fresh root
// Router, flattens to an *http.ServeMux, and returns it. Global
// middleware should be applied by the caller if needed.
func BuildHandler(controllers ...Controller) http.Handler {
	root := &router{}
	for _, c := range controllers {
		c.Routes(root)
	}
	mux := http.NewServeMux()
	root.build(mux, nil)
	return mux
}

type router struct {
	prefix      string
	parent      *router
	middlewares []Middleware
	routes      []routeEntry
	mounts      []mountEntry
	children    []*router
}

type routeEntry struct {
	method, path string
	handler      http.Handler
}

type mountEntry struct {
	prefix  string
	handler http.Handler
}

func (r *router) Route(prefix string, fn func(Router)) {
	sub := &router{
		prefix: joinPath(r.prefix, prefix),
		parent: r,
	}
	fn(sub)
	r.children = append(r.children, sub)
}

func (r *router) With(mw ...Middleware) Router {
	sub := &router{
		prefix:      r.prefix,
		parent:      r,
		middlewares: append([]Middleware(nil), mw...),
	}
	r.children = append(r.children, sub)
	return sub
}

func (r *router) Use(mw ...Middleware) {
	r.middlewares = append(r.middlewares, mw...)
}

func (r *router) Get(path string, h http.Handler)    { r.Method(http.MethodGet, path, h) }
func (r *router) Post(path string, h http.Handler)   { r.Method(http.MethodPost, path, h) }
func (r *router) Put(path string, h http.Handler)    { r.Method(http.MethodPut, path, h) }
func (r *router) Patch(path string, h http.Handler)  { r.Method(http.MethodPatch, path, h) }
func (r *router) Delete(path string, h http.Handler) { r.Method(http.MethodDelete, path, h) }

func (r *router) Method(method, path string, h http.Handler) {
	r.routes = append(r.routes, routeEntry{
		method:  method,
		path:    joinPath(r.prefix, path),
		handler: h,
	})
}

func (r *router) Mount(prefix string, h http.Handler) {
	r.mounts = append(r.mounts, mountEntry{
		prefix:  joinPath(r.prefix, prefix),
		handler: h,
	})
}

// build walks the router tree and registers every leaf route on the
// supplied mux, wrapping each handler in its accumulated middleware
// stack (root-most middleware applied outermost).
//
// When collector is non-nil, each leaf route also feeds the collector
// so the OpenAPI spec reflects every registered operation. Mount
// entries are skipped — mounted sub-handlers (file servers, third-
// party routers) don't have a known schema for us to extract.
func (r *router) build(mux *http.ServeMux, collector *openapi.Collector) {
	mw := r.accumulatedMiddlewares()
	for _, e := range r.routes {
		mux.Handle(fmt.Sprintf("%s %s", e.method, e.path), wrapOuterFirst(e.handler, mw))
		if collector != nil {
			// The Collector skips paths it was told to skip
			// (kit infrastructure: spec endpoint, UI, health
			// probes). Anything else is offered to the spec.
			if err := collector.CollectOperation(e.method, e.path, e.handler); err != nil {
				fmt.Fprintf(os.Stderr, "rest: openapi collection failed for %s %s: %v\n", e.method, e.path, err)
			}
		}
	}
	for _, m := range r.mounts {
		// StripPrefix so the mounted handler sees a request path
		// relative to its mount point — same semantics as chi.Mount.
		mux.Handle(m.prefix+"/", wrapOuterFirst(http.StripPrefix(strings.TrimRight(m.prefix, "/"), m.handler), mw))
	}
	for _, child := range r.children {
		child.build(mux, collector)
	}
}

// accumulatedMiddlewares returns the middleware stack visible at
// this router scope, walking root-to-leaf so the root's middleware
// appears first in the slice (outermost).
func (r *router) accumulatedMiddlewares() []Middleware {
	if r.parent == nil {
		return append([]Middleware(nil), r.middlewares...)
	}
	parent := r.parent.accumulatedMiddlewares()
	return append(parent, r.middlewares...)
}

// wrapOuterFirst wraps h such that mws[0] runs first (outermost),
// mws[len-1] runs last before the handler. Mirrors the natural
// reading order of r.Use(auth, actor, tenancy) — auth wraps
// everything else.
func wrapOuterFirst(h http.Handler, mws []Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i].Intercept(h)
	}
	return h
}

// joinPath concatenates two URL path segments with exactly one "/"
// between them. Empty segments are ignored. Always returns a path
// that starts with "/" (or "" if both inputs are empty).
func joinPath(prefix, suffix string) string {
	if suffix == "" {
		if prefix == "" {
			return ""
		}
		return prefix
	}
	if prefix == "" {
		if strings.HasPrefix(suffix, "/") {
			return suffix
		}
		return "/" + suffix
	}
	p := strings.TrimRight(prefix, "/")
	s := suffix
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	return p + s
}
