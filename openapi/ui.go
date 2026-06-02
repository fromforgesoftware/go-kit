package openapi

import (
	"net/http"

	"github.com/fromforgesoftware/go-kit/openapi/internal/swaggestimpl"
)

// UIRenderer identifies which interactive documentation UI to serve.
// Default is SwaggerUI; alternatives are Redoc and StoplightElements
// (currently both fall back to Swagger UI; static assets for the
// alternative renderers can be plugged in later without changing
// the public API).
type UIRenderer string

const (
	// SwaggerUI renders Swagger UI v5 with Authorize button,
	// "Try it out", and the server dropdown.
	SwaggerUI UIRenderer = "swagger-ui"

	// Redoc renders Redoc's three-pane reference layout. Read-only.
	Redoc UIRenderer = "redoc"

	// StoplightElements renders Stoplight Elements (a Redoc-style
	// reference with optional try-it-out).
	StoplightElements UIRenderer = "stoplight"
)

// NewUIHandler returns an http.Handler that renders the documentation
// UI at the given basePath, pointing at the supplied specURL.
//
// All three renderer choices currently route through Swagger UI v5;
// Redoc and Stoplight are placeholders. The handler degrades to
// Swagger UI rather than 404 so consumer configuration remains valid.
func NewUIHandler(title, specURL, basePath string, renderer UIRenderer) http.Handler {
	_ = renderer
	return swaggestimpl.NewSwaggerUIHandler(title, specURL, basePath)
}
