package rest

import (
	"context"
	"io"
	"net/http"

	"github.com/fromforgesoftware/go-kit/jsonapi"
)

// WriteJSONAPI marshals model as a single-resource jsonapi document
// and writes it to w with the given status. Uses the same
// buffer-first-then-flush pattern the kit handlers use so a marshal
// failure can still produce a clean JSON:API error (no torn response).
//
// Exported as an escape hatch for handlers that don't fit
// Create/Get/List/Patch/Update/Delete or Command/Bulk shapes — rare,
// but real for things like multi-resource composite endpoints or
// webhook receivers that need to return a small ack.
func WriteJSONAPI(w http.ResponseWriter, status int, model any) error {
	return writeBuffered(w, "application/vnd.api+json; charset=utf-8", status, func(buf io.Writer) error {
		return jsonapi.MarshalPayload(buf, model)
	})
}

// WriteJSONAPIError encodes err as a jsonapi error document and
// writes it to w with the appropriate status. Thin wrapper around
// JsonApiErrorEncoder for handlers that need to emit an error from
// outside the kit handler factories.
func WriteJSONAPIError(w http.ResponseWriter, err error) {
	JsonApiErrorEncoder(context.Background(), err, w)
}
