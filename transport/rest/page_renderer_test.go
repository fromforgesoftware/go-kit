package rest_test

import (
	"context"
	"errors"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/transport/rest"
)

func parsedPages(t *testing.T) *template.Template {
	t.Helper()
	return template.Must(template.New("hello.html").Parse(`<h1>{{.Greeting}}</h1>`))
}

func TestPageRenderer_Render(t *testing.T) {
	rr := httptest.NewRecorder()
	r := rest.NewPageRenderer(parsedPages(t))

	require.NoError(t, r.Render(rr, http.StatusOK, "hello.html", map[string]string{"Greeting": "hi"}))
	res := rr.Result()
	t.Cleanup(func() { _ = res.Body.Close() })

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", res.Header.Get("Content-Type"))
	assert.NotEmpty(t, res.Header.Get("Content-Length"))
	body, _ := io.ReadAll(res.Body)
	assert.Equal(t, "<h1>hi</h1>", string(body))
}

func TestPageRenderer_RenderAttachesCookies(t *testing.T) {
	rr := httptest.NewRecorder()
	r := rest.NewPageRenderer(parsedPages(t))

	require.NoError(t, r.Render(rr, 0, "hello.html", map[string]string{"Greeting": "hi"},
		&http.Cookie{Name: "sid", Value: "abc", HttpOnly: true, Path: "/"}))

	res := rr.Result()
	t.Cleanup(func() { _ = res.Body.Close() })
	cookies := res.Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "sid", cookies[0].Name)
	assert.Equal(t, "abc", cookies[0].Value)
}

type failingTemplate struct{}

func (failingTemplate) ExecuteTemplate(io.Writer, string, any) error {
	return errors.New("template blew up")
}

func TestPageRenderer_RenderFailureLeavesResponseUntouched(t *testing.T) {
	// A template failure must surface as an error BEFORE any byte hits the
	// wire, so callers can answer with a clean 500 instead of a half-rendered
	// body under a 200 header.
	rr := httptest.NewRecorder()
	r := rest.NewPageRenderer(failingTemplate{})

	err := r.Render(rr, http.StatusOK, "anything", nil)
	require.Error(t, err)
	res := rr.Result()
	t.Cleanup(func() { _ = res.Body.Close() })
	body, _ := io.ReadAll(res.Body)
	assert.Equal(t, http.StatusOK, res.StatusCode, "httptest default; the renderer didn't call WriteHeader")
	assert.Empty(t, strings.TrimSpace(string(body)), "no body should have been written")
}

func TestPageRenderer_Redirect(t *testing.T) {
	rr := httptest.NewRecorder()
	r := rest.NewPageRenderer(parsedPages(t))

	r.Redirect(rr, 0, "/next", &http.Cookie{Name: "sid", Value: "v", Path: "/"})

	res := rr.Result()
	t.Cleanup(func() { _ = res.Body.Close() })
	assert.Equal(t, http.StatusFound, res.StatusCode)
	assert.Equal(t, "/next", res.Header.Get("Location"))
	require.Len(t, res.Cookies(), 1)
	assert.Equal(t, "sid", res.Cookies()[0].Name)
}

func TestPageRenderer_RedirectCustomStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	r := rest.NewPageRenderer(parsedPages(t))

	r.Redirect(rr, http.StatusSeeOther, "/next")

	res := rr.Result()
	t.Cleanup(func() { _ = res.Body.Close() })
	assert.Equal(t, http.StatusSeeOther, res.StatusCode)
}

type greetInput struct{ Name string }

func decodeGreet(_ context.Context, r *http.Request) (any, error) {
	return greetInput{Name: r.URL.Query().Get("name")}, nil
}

func TestNewPageHandler_RendersTemplate(t *testing.T) {
	renderer := rest.NewPageRenderer(parsedPages(t))
	h := rest.NewPageHandler(renderer, decodeGreet,
		func(_ context.Context, in greetInput) (rest.PageResult, error) {
			return rest.PageResult{Template: "hello.html", Data: map[string]string{"Greeting": in.Name}}, nil
		},
	)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/?name=alice", nil))
	res := rr.Result()
	t.Cleanup(func() { _ = res.Body.Close() })

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", res.Header.Get("Content-Type"))
	body, _ := io.ReadAll(res.Body)
	assert.Equal(t, "<h1>alice</h1>", string(body))
}

func TestNewPageHandler_Redirects(t *testing.T) {
	renderer := rest.NewPageRenderer(parsedPages(t))
	h := rest.NewPageHandler(renderer, decodeGreet,
		func(_ context.Context, _ greetInput) (rest.PageResult, error) {
			return rest.PageResult{
				RedirectTo: "/landing",
				Cookies:    []*http.Cookie{{Name: "sid", Value: "v", Path: "/"}},
			}, nil
		},
	)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/submit", nil))
	res := rr.Result()
	t.Cleanup(func() { _ = res.Body.Close() })

	assert.Equal(t, http.StatusFound, res.StatusCode)
	assert.Equal(t, "/landing", res.Header.Get("Location"))
	require.Len(t, res.Cookies(), 1)
	assert.Equal(t, "sid", res.Cookies()[0].Name)
}
