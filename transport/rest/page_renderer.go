package rest

import (
	"context"
	"io"
	"net/http"

	"github.com/fromforgesoftware/go-kit/transport"
)

// PageTemplate is the minimum surface PageRenderer needs from the template
// engine. *html/template.Template satisfies it via ExecuteTemplate.
type PageTemplate interface {
	ExecuteTemplate(w io.Writer, name string, data any) error
}

// PageRenderer renders HTML pages with the same buffered-write safety the
// kit's JSON encoders use: a template-render failure becomes a clean error
// before any byte hits the wire, instead of a torn body under a 200 header.
// It also factors out the cookie + redirect plumbing the hosted-page flow
// repeats per route.
type PageRenderer struct {
	tmpl PageTemplate
}

func NewPageRenderer(tmpl PageTemplate) *PageRenderer {
	return &PageRenderer{tmpl: tmpl}
}

// Render executes the named template into a pooled buffer, then writes the
// response in one shot (Content-Type, Content-Length, status, body). Any
// cookies are attached before the status header so they reach the client.
// Returns the template error so callers can decide between re-render and 500.
func (r *PageRenderer) Render(w http.ResponseWriter, status int, name string, data any, cookies ...*http.Cookie) error {
	for _, c := range cookies {
		http.SetCookie(w, c)
	}
	if status == 0 {
		status = http.StatusOK
	}
	return writeBuffered(w, "text/html; charset=utf-8", status, func(buf io.Writer) error {
		return r.tmpl.ExecuteTemplate(buf, name, data)
	})
}

// Redirect issues an HTTP redirect (302 by default) to location, attaching
// any cookies first.
func (r *PageRenderer) Redirect(w http.ResponseWriter, status int, location string, cookies ...*http.Cookie) {
	for _, c := range cookies {
		http.SetCookie(w, c)
	}
	if status == 0 {
		status = http.StatusFound
	}
	w.Header().Set("Location", location)
	w.WriteHeader(status)
}

// PageResult is what an HTML-page endpoint returns: either a template render
// (Template + Data) or a redirect (RedirectTo), optionally setting cookies.
// One PageResult covers both outcomes so handlers don't need bespoke
// "render vs redirect" branching on the wire side.
type PageResult struct {
	Template   string
	Status     int
	Data       any
	Cookies    []*http.Cookie
	RedirectTo string
}

// NewPageHandler composes a hosted-page handler the same way the JSON:API
// factories compose API handlers: a decoder lifts the request into an input
// I, an endpoint produces a PageResult, and the renderer writes it. Render
// outcomes use the buffered-write safety; redirect outcomes attach cookies
// before the Location header.
//
// Use this when the page is shape "decode → call usecase → render/redirect."
// Pages with branching side-effects can still call PageRenderer directly.
func NewPageHandler[I any](
	renderer *PageRenderer,
	decoder DecodeRequestFunc,
	endpoint transport.Endpoint[I, PageResult],
	opts ...HandlerOpt,
) http.Handler {
	encoder := NewHTTPEncoder(func(_ context.Context, w http.ResponseWriter, res PageResult) error {
		if res.RedirectTo != "" {
			renderer.Redirect(w, res.Status, res.RedirectTo, res.Cookies...)
			return nil
		}
		return renderer.Render(w, res.Status, res.Template, res.Data, res.Cookies...)
	})
	return NewHandler(endpoint, decoder, encoder, append([]HandlerOpt{HandlerAllowsEmptyReq(true)}, opts...)...)
}
