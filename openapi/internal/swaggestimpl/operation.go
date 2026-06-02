package swaggestimpl

import (
	"github.com/swaggest/openapi-go/openapi31"
)

// ExposeOperation reaches through swaggest's OperationExposer (openapi31
// variant) to get the concrete *Operation for direct mutation. Used by
// the forge wrapper to attach external docs and example values that
// aren't exposed on the public OperationContext interface.
func ExposeOperation(oc OperationContext) *openapi31.Operation {
	if e, ok := oc.(openapi31.OperationExposer); ok {
		return e.Operation()
	}
	return nil
}

// ApplyOperationExternalDocs attaches an external docs block to the
// operation.
func ApplyOperationExternalDocs(op *openapi31.Operation, url, description string) {
	if op == nil {
		return
	}
	ed := op.ExternalDocsEns()
	ed.URL = url
	if description != "" {
		d := description
		ed.Description = &d
	}
}

// AddRequestExample attaches a named example to every content-type
// declared on the operation's request body. Called after the request
// structure has been added so the body's media types are populated.
//
// Idempotent on the name: subsequent calls with the same name
// overwrite the prior example.
func AddRequestExample(op *openapi31.Operation, name string, value interface{}) {
	if op == nil || op.RequestBody == nil || op.RequestBody.RequestBody == nil {
		return
	}
	for ct, mt := range op.RequestBody.RequestBody.Content {
		ex := openapi31.Example{}
		v := value
		ex.WithValue(&v)
		mt.WithExamplesItem(name, openapi31.ExampleOrReference{Example: &ex})
		op.RequestBody.RequestBody.Content[ct] = mt
	}
}

// AddResponseExample attaches a named example to every content-type of
// the response at the given HTTP status. Like AddRequestExample, the
// status entry must already exist (added via AddRespStructure).
func AddResponseExample(op *openapi31.Operation, status int, name string, value interface{}) {
	if op == nil || op.Responses == nil {
		return
	}
	key := statusKey(status)
	resp, ok := op.Responses.MapOfResponseOrReferenceValues[key]
	if !ok || resp.Response == nil {
		return
	}
	for ct, mt := range resp.Response.Content {
		ex := openapi31.Example{}
		v := value
		ex.WithValue(&v)
		mt.WithExamplesItem(name, openapi31.ExampleOrReference{Example: &ex})
		resp.Response.Content[ct] = mt
	}
	op.Responses.MapOfResponseOrReferenceValues[key] = resp
}

func statusKey(status int) string {
	// swaggest stores response keys as "200", "201", "default", …
	// — bare integer string, no leading zeros.
	switch {
	case status >= 100 && status <= 599:
		return itoa(status)
	default:
		return "default"
	}
}

func itoa(n int) string {
	// tiny stand-in to avoid an strconv import for one call. Spec
	// statuses are always 3 digits in OpenAPI.
	const digits = "0123456789"
	buf := [3]byte{}
	for i := 2; i >= 0; i-- {
		buf[i] = digits[n%10]
		n /= 10
	}
	return string(buf[:])
}

// EnsurePathParameter appends a Parameter{In: path, Name: name,
// Required: true} to the operation if one isn't already declared.
// Used by the Collector to satisfy swaggest's path-param validation
// when the kit's handlers don't declare path params via struct tags
// (they use r.PathValue("name") instead).
func EnsurePathParameter(op *openapi31.Operation, name string) {
	if op == nil || name == "" {
		return
	}
	for _, p := range op.Parameters {
		if p.Parameter != nil && p.Parameter.In == openapi31.ParameterInPath && p.Parameter.Name == name {
			return
		}
	}
	required := true
	p := openapi31.Parameter{
		Name:     name,
		In:       openapi31.ParameterInPath,
		Required: &required,
	}
	// Path params need a schema or content map; the simplest schema
	// is a string, which matches stdlib mux's any-character-but-slash
	// semantics.
	p.WithSchema(map[string]any{"type": "string"})
	op.Parameters = append(op.Parameters, openapi31.ParameterOrReference{Parameter: &p})
}
