// Package openapi is forge's owned wrapper around the underlying
// OpenAPI 3.1 reflection engine.
//
// Callers (the rest transport, the JSON:API helpers, the petstore
// example, downstream services) reference only the types defined in
// this package; the swaggest/openapi-go dependency lives behind
// internal/swaggestimpl. Replacing or vendoring the implementation
// changes only that internal package.
//
// Two distinct option families:
//
//   - SpecOpt — server/spec-level configuration (title, version,
//     servers, security schemes, default security). Applied via
//     rest.WithOpenAPI(...).
//
//   - OpOpt  — per-operation metadata (summary, tags, deprecated,
//     errors, security override). Applied via rest.HandlerWithOpenAPI(...).
//
// The Collector is the orchestrator: it walks every leaf route the
// kit's Router registers, asks the handler to describe itself via
// the Preparer interface, applies any operation annotations, and
// emits the resulting OpenAPI 3.1 spec at the configured route.
package openapi
