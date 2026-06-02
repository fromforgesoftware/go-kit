package swaggestimpl

import (
	"net/http"

	"github.com/swaggest/swgui/v5emb"
)

// NewSwaggerUIHandler returns an http.Handler that renders Swagger UI
// v5 at basePath, fetching its spec from specURL. Title shows in the
// UI's header.
func NewSwaggerUIHandler(title, specURL, basePath string) http.Handler {
	return v5emb.New(title, specURL, basePath)
}
