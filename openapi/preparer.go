package openapi

// Preparer is implemented by HTTP handlers that can describe
// themselves to the OpenAPI Collector. Every kit handler factory
// emits a value that satisfies Preparer, so the Collector can walk
// the registered routes and produce the spec without any annotations
// in code comments.
type Preparer interface {
	SetupOperation(oc OperationContext) error
}
