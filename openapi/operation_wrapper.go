package openapi

import (
	"github.com/fromforgesoftware/go-kit/openapi/internal/swaggestimpl"
)

// operationContext is the concrete OperationContext returned by the
// default Reflector. It wraps a swaggest OperationContext and exposes
// the forge-owned method set.
//
// Examples are buffered on the value rather than written through to
// the underlying operation immediately, because swaggest doesn't
// populate Operation.RequestBody / Operation.Responses content maps
// until AddOperation is called. The Collector calls applyDeferred
// just before AddOperation to flush the buffer.
type operationContext struct {
	oc swaggestimpl.OperationContext

	reqExamples  []bufferedExample
	respExamples []bufferedExample
}

type bufferedExample struct {
	status int // -1 for request examples
	name   string
	value  interface{}
}

func (o *operationContext) AddRequest(body any) {
	o.oc.AddReqStructure(body)
}

func (o *operationContext) AddResponse(status int, body any) {
	o.oc.AddRespStructure(body, swaggestimpl.WithHTTPStatus(status))
}

func (o *operationContext) SetSummary(s string) {
	o.oc.SetSummary(s)
}

func (o *operationContext) SetDescription(s string) {
	o.oc.SetDescription(s)
}

func (o *operationContext) SetTags(tags ...string) {
	o.oc.SetTags(tags...)
}

func (o *operationContext) SetOperationID(id string) {
	o.oc.SetID(id)
}

func (o *operationContext) SetDeprecated(b bool) {
	o.oc.SetIsDeprecated(b)
}

func (o *operationContext) AddRequestExample(name string, example any) {
	o.reqExamples = append(o.reqExamples, bufferedExample{name: name, value: example})
}

func (o *operationContext) AddResponseExample(status int, name string, example any) {
	o.respExamples = append(o.respExamples, bufferedExample{status: status, name: name, value: example})
}

// pendingExamples returns the buffered examples for the Collector to
// apply after AddOperation. Keeps operation_wrapper.go free of any
// direct swaggest references.
func (o *operationContext) pendingExamples() ([]bufferedExample, []bufferedExample) {
	return o.reqExamples, o.respExamples
}

func (o *operationContext) SetSecurity(schemes ...string) {
	op := swaggestimpl.ExposeOperation(o.oc)
	if op == nil {
		return
	}
	// The OpenAPI 3.1 spec uses `security: []` on an operation to mean
	// "no security required, overriding any root-level default". The
	// underlying Operation struct tags Security with omitempty, which
	// drops both nil and len-0 slices on marshal. To emit the override
	// we materialise a slice containing one empty requirement map —
	// `[{}]`. Spec consumers (Swagger UI, OpenAPI Generator, redocly)
	// treat an empty requirement set identically to `[]`: no scheme
	// needed for this operation.
	if len(schemes) == 0 {
		op.Security = []map[string][]string{{}}
		return
	}
	req := map[string][]string{}
	for _, name := range schemes {
		req[name] = []string{}
	}
	op.Security = append([]map[string][]string(nil), req)
}

func (o *operationContext) AddExternalDocs(url, description string) {
	op := swaggestimpl.ExposeOperation(o.oc)
	if op == nil {
		return
	}
	swaggestimpl.ApplyOperationExternalDocs(op, url, description)
}

// ensurePathParameter is called by the Collector to inject path params
// derived from the URL pattern. Unexported because callers shouldn't
// declare path params manually — the kit handlers don't know they
// have any, the Collector does.
func (o *operationContext) ensurePathParameter(name string) {
	op := swaggestimpl.ExposeOperation(o.oc)
	swaggestimpl.EnsurePathParameter(op, name)
}
