package jsonapi

// This file contains schema-only types: the OpenAPI reflector consumes
// them to emit JSON:API envelope schemas, but they're never used at
// runtime (the actual marshalling lives in node.go / response.go).
//
// They're kept here, next to the runtime types, so the wire shape and
// the schema shape stay in sync. If you change the envelope, change
// both.

// Document is the JSON:API 1.1 single-resource envelope.
type Document[T any] struct {
	Data     T                 `json:"data"`
	Included []map[string]any  `json:"included,omitempty"`
	Links    map[string]string `json:"links,omitempty"`
	Meta     map[string]any    `json:"meta,omitempty"`
}

// ListDocument is the JSON:API 1.1 multi-resource envelope.
type ListDocument[T any] struct {
	Data     []T               `json:"data"`
	Included []map[string]any  `json:"included,omitempty"`
	Links    map[string]string `json:"links,omitempty"`
	Meta     map[string]any    `json:"meta,omitempty"`
}

// ErrorDocument is the JSON:API 1.1 error envelope. Carries one or
// more ErrorObject entries (defined in errors.go).
type ErrorDocument struct {
	Errors []ErrorObject `json:"errors"`
}

// ResourceEnvelope mirrors the `data` sub-object of a JSON:API
// document for resource types that carry attributes + relationships
// separately. Most callers won't reference this directly — Document[T]
// already wraps a fully-typed DTO that has its own jsonapi struct
// tags — but the helper schema is here for handlers that bypass the
// generic envelope (e.g. action endpoints with no resource shape).
type ResourceEnvelope[A any] struct {
	Type          string            `json:"type"`
	ID            string            `json:"id,omitempty"`
	Attributes    A                 `json:"attributes,omitempty"`
	Relationships map[string]any    `json:"relationships,omitempty"`
	Links         map[string]string `json:"links,omitempty"`
	Meta          map[string]any    `json:"meta,omitempty"`
}
