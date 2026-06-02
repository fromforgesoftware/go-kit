// Package swaggestimpl is the sole importer of github.com/swaggest/openapi-go
// in the forge repo. It exposes type aliases and small constructors that
// other forge packages (openapi/, transport/rest/, jsonapi/, cmd/petstore/)
// use without ever pulling swaggest into their import graphs.
//
// Keeping the boundary tight makes it trivial to replace or vendor the
// implementation later — only this package changes.
package swaggestimpl

import (
	"reflect"
	"strings"

	"github.com/swaggest/jsonschema-go"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi31"
)

// Reflector aliases swaggest's reflector interface. Both openapi3
// and openapi31 reflectors implement it; we use the openapi31 variant
// by default for OpenAPI 3.1 output.
type Reflector = openapi.Reflector

// OperationContext aliases swaggest's per-operation context. Method
// names from the swaggest interface (AddReqStructure, SetSummary,
// etc.) are wrapped on the openapi side; callers reference only the
// forge-owned openapi.OperationContext interface.
type OperationContext = openapi.OperationContext

// Spec31 aliases the OpenAPI 3.1 spec root type. The forge wrapper
// mutates this directly to set info / servers / tags / security
// schemes / default security.
type Spec31 = openapi31.Spec

// ContentOption aliases swaggest's content-option type used by
// AddReqStructure and AddRespStructure.
type ContentOption = openapi.ContentOption

// WithHTTPStatus is the canonical content option for declaring the
// response status of an AddRespStructure call.
func WithHTTPStatus(status int) ContentOption {
	return openapi.WithHTTPStatus(status)
}

// NewReflector returns a fresh openapi31 reflector — the canonical
// engine for OpenAPI 3.1 output.
//
// Configured to strip kit package prefixes from schema names so
// `openapi.ErrorBody` doesn't surface as `OpenapiErrorBody` and
// `jsonapi.Document` doesn't surface as `JsonapiDocument` in the
// `#/components/schemas/...` map. Consumer-defined types keep their
// natural prefix (`auth.UserDTO` stays `AuthUserDTO`).
func NewReflector() Reflector {
	r := openapi31.NewReflector()
	r.JSONSchemaReflector().DefaultOptions = append(
		r.JSONSchemaReflector().DefaultOptions,
		jsonschema.InterceptDefName(stripKitPrefix),
	)
	return r
}

// stripKitPrefix removes the camelcased forge-kit package prefix from
// a default definition name. Operates on the already-derived default
// name produced by jsonschema-go.
func stripKitPrefix(_ reflect.Type, defaultDefName string) string {
	for _, prefix := range []string{"Openapi", "Jsonapi", "Resttransport", "Resource"} {
		if s, ok := trim(defaultDefName, prefix); ok {
			return s
		}
	}
	return defaultDefName
}

func trim(s, prefix string) (string, bool) {
	rest := strings.TrimPrefix(s, prefix)
	if rest == s || rest == "" {
		return s, false
	}
	return rest, true
}

// Spec returns the underlying *openapi31.Spec for direct mutation.
// Used by the wrapper to apply spec-level configuration that doesn't
// have an exposed helper on the swaggest Reflector interface.
func Spec(r Reflector) *Spec31 {
	return r.SpecSchema().(*Spec31)
}

// OperationOn looks up an Operation already added to the spec by
// (method, path). Returns nil if no such operation exists. The
// Collector uses this to apply example mutations that have to land
// AFTER AddOperation has finalised the Operation's RequestBody and
// Responses content maps.
func OperationOn(r Reflector, method, path string) *openapi31.Operation {
	spec := Spec(r)
	if spec.Paths == nil {
		return nil
	}
	item, ok := spec.Paths.MapOfPathItemValues[path]
	if !ok {
		return nil
	}
	op, err := item.Operation(method)
	if err != nil {
		return nil
	}
	return op
}
