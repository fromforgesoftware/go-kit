package openapi

// OperationContext is the surface a handler talks to when describing
// itself for the OpenAPI spec. The kit's handler factories implement
// Preparer and call AddRequest / AddResponse / setters on this type
// during spec collection.
type OperationContext interface {
	// AddRequest declares the request body shape. Pass a zero value
	// of the request struct; the underlying reflector walks its
	// json/jsonapi tags to produce the JSON Schema.
	AddRequest(body any)

	// AddResponse declares a response for the given HTTP status.
	// May be called multiple times to declare error responses.
	AddResponse(status int, body any)

	// SetSummary sets the operation's short, single-line summary.
	SetSummary(s string)

	// SetDescription sets the operation's longer prose description.
	SetDescription(s string)

	// SetTags groups the operation under one or more tags. The UI
	// uses these to organise the operation list.
	SetTags(tags ...string)

	// SetOperationID sets a globally-unique operation ID. If unset,
	// the reflector derives one from method+path.
	SetOperationID(id string)

	// SetDeprecated marks the operation as deprecated.
	SetDeprecated(b bool)

	// AddRequestExample attaches a named example to the request body.
	AddRequestExample(name string, example any)

	// AddResponseExample attaches a named example to a specific
	// response status.
	AddResponseExample(status int, name string, example any)

	// SetSecurity requires the operation be authorised by all the
	// named schemes (AND semantics). Schemes must be declared at
	// spec level via SecurityScheme. Pass no arguments to explicitly
	// clear any default security (i.e. mark the operation public).
	SetSecurity(schemes ...string)

	// AddExternalDocs attaches a single external documentation link
	// to the operation.
	AddExternalDocs(url, description string)
}

// OpOpt is a per-operation option. The kit composes a slice of these
// into a single function that runs against the OperationContext during
// spec collection.
type OpOpt func(OperationContext) error

// Summary sets the operation summary.
func Summary(s string) OpOpt {
	return func(oc OperationContext) error {
		oc.SetSummary(s)
		return nil
	}
}

// Description sets the operation description.
func Description(s string) OpOpt {
	return func(oc OperationContext) error {
		oc.SetDescription(s)
		return nil
	}
}

// Tags groups the operation under the given tags.
func Tags(tags ...string) OpOpt {
	return func(oc OperationContext) error {
		oc.SetTags(tags...)
		return nil
	}
}

// OperationID overrides the auto-derived operation ID.
func OperationID(id string) OpOpt {
	return func(oc OperationContext) error {
		oc.SetOperationID(id)
		return nil
	}
}

// Deprecated marks the operation as deprecated.
func Deprecated() OpOpt {
	return func(oc OperationContext) error {
		oc.SetDeprecated(true)
		return nil
	}
}

// Errors is a shorthand for declaring several error responses
// using the kit's standard ErrorBody schema. Each status maps to a
// response with the same body shape.
func Errors(statuses ...int) OpOpt {
	return func(oc OperationContext) error {
		for _, status := range statuses {
			oc.AddResponse(status, ErrorBody{})
		}
		return nil
	}
}

// Response declares an additional response with an arbitrary body.
// Use when the body for a status code differs from the kit standard
// ErrorBody.
func Response(status int, body any) OpOpt {
	return func(oc OperationContext) error {
		oc.AddResponse(status, body)
		return nil
	}
}

// RequestExample attaches a named example to the request body.
func RequestExample(name string, example any) OpOpt {
	return func(oc OperationContext) error {
		oc.AddRequestExample(name, example)
		return nil
	}
}

// ResponseExample attaches a named example to a specific response
// status.
func ResponseExample(status int, name string, example any) OpOpt {
	return func(oc OperationContext) error {
		oc.AddResponseExample(status, name, example)
		return nil
	}
}

// Security requires the operation be authorised by all the named
// schemes. The schemes must be declared at spec level via
// SecurityScheme.
func Security(schemeNames ...string) OpOpt {
	return func(oc OperationContext) error {
		oc.SetSecurity(schemeNames...)
		return nil
	}
}

// NoSecurity explicitly clears any default security on the operation,
// marking it public. Used to override DefaultSecurity for a single
// route (e.g. POST /login).
func NoSecurity() OpOpt {
	return func(oc OperationContext) error {
		oc.SetSecurity()
		return nil
	}
}

// ExternalDocs attaches a single external documentation link.
func ExternalDocs(url, description string) OpOpt {
	return func(oc OperationContext) error {
		oc.AddExternalDocs(url, description)
		return nil
	}
}

// Raw is an escape hatch for callers that need to manipulate the
// OperationContext directly. Prefer the typed helpers above.
func Raw(fn func(oc OperationContext) error) OpOpt {
	return fn
}

// ErrorBody is the canonical error response shape emitted by the
// kit's default plain-JSON error encoder. Op-level Errors() declares
// responses with this body.
type ErrorBody struct {
	Error  string `json:"error"`
	Status int    `json:"status"`
}
