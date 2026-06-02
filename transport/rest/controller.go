package rest

import "net/http"

// IDPath is the conventional URL segment for a resource id, used in
// Router registrations like r.Get(IDPath, h) when callers want the
// pattern to match the kit's "/{id}" convention exactly.
const IDPath = "/{id}"

// Controller declares HTTP routes for an aggregate. The Routes method
// receives a Router and registers grouped routes + scoped middleware
// on it. fx-wired controllers are collected via NewFxController and
// invoked at NewServer time against the server's root router.
//
//	func (c *workspaceController) Routes(r rest.Router) {
//	    r.Route("/v1/workspaces", func(r rest.Router) {
//	        r.Use(c.auth, c.actor)
//	        r.Post("/", c.create)
//	        r.Route("/{id}", func(r rest.Router) {
//	            r.Use(c.tenancy)
//	            r.Delete("/", c.delete)
//	        })
//	    })
//	}
type Controller interface {
	Routes(r Router)
}

// Endpoint is a (method, path, handler) triple used by WithEndpoints
// to register singleton routes that don't belong to a Controller —
// /healthz, /readyz, or any handler attached at the server level.
type Endpoint interface {
	Method() string
	Path() string
	http.Handler
}

type endpoint struct {
	method string
	path   string
	http.Handler
}

func (e *endpoint) Method() string { return e.method }
func (e *endpoint) Path() string   { return e.path }

func NewEndpoint(method, path string, handler http.Handler) Endpoint {
	return &endpoint{method: method, path: path, Handler: handler}
}
