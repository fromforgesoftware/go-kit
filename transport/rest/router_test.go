package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubHandler returns a handler that records its invocation by writing
// the given marker to the response. Used to assert which leaf got
// hit and in what middleware order.
func stubHandler(marker string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(marker))
	})
}

// recordingMW prepends its tag to whatever the inner handler wrote,
// proving that wrapping ran in the expected order (outer mw appends
// first, then the handler's body).
func recordingMW(tag string) Middleware {
	return MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(tag + ">"))
			next.ServeHTTP(w, r)
		})
	})
}

func TestRouterFlattensSingleRoute(t *testing.T) {
	r := NewRouter().(*router)
	r.Get("/healthz", stubHandler("ok"))

	mux := http.NewServeMux()
	r.build(mux, nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Body.String() != "ok" {
		t.Fatalf("expected body 'ok', got %q", rec.Body.String())
	}
}

func TestRouterNestedRouteJoinsPrefixes(t *testing.T) {
	r := NewRouter().(*router)
	r.Route("/v1", func(r Router) {
		r.Route("/workspaces", func(r Router) {
			r.Get("/{id}", stubHandler("ws-get"))
			r.Post("/", stubHandler("ws-create"))
		})
	})

	mux := http.NewServeMux()
	r.build(mux, nil)

	cases := []struct {
		method, path, want string
	}{
		{http.MethodGet, "/v1/workspaces/abc", "ws-get"},
		{http.MethodPost, "/v1/workspaces/", "ws-create"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Body.String() != c.want {
			t.Errorf("%s %s: want %q, got %q", c.method, c.path, c.want, rec.Body.String())
		}
	}
}

func TestRouterUseAppliesMiddlewareOuterFirst(t *testing.T) {
	// Use(a, b, c) → a runs first (outermost), c runs last before handler.
	// Recorded output should read "a>b>c>handler".
	r := NewRouter().(*router)
	r.Use(recordingMW("a"), recordingMW("b"), recordingMW("c"))
	r.Get("/x", stubHandler("h"))

	mux := http.NewServeMux()
	r.build(mux, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if got, want := rec.Body.String(), "a>b>c>h"; got != want {
		t.Fatalf("middleware order: want %q, got %q", want, got)
	}
}

func TestRouterUseInheritedByNestedRoute(t *testing.T) {
	// Root Use should wrap every leaf below, regardless of nesting.
	r := NewRouter().(*router)
	r.Use(recordingMW("root"))
	r.Route("/inner", func(r Router) {
		r.Use(recordingMW("inner"))
		r.Get("/x", stubHandler("h"))
	})

	mux := http.NewServeMux()
	r.build(mux, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/inner/x", nil))
	if got, want := rec.Body.String(), "root>inner>h"; got != want {
		t.Fatalf("nested mw order: want %q, got %q", want, got)
	}
}

func TestRouterWithScopesMiddlewareToOnlyThatRegistration(t *testing.T) {
	// With(mw) should not leak mw to sibling routes registered on the
	// parent. Critical for endpoints like "POST /create is public,
	// PATCH /{id} is auth-gated" registered on the same router.
	r := NewRouter().(*router)
	r.Route("/r", func(r Router) {
		r.Get("/public", stubHandler("pub"))
		r.With(recordingMW("auth")).Get("/private", stubHandler("priv"))
	})

	mux := http.NewServeMux()
	r.build(mux, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/r/public", nil))
	if got := rec.Body.String(); got != "pub" {
		t.Errorf("public leaked mw: got %q", got)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/r/private", nil))
	if got, want := rec.Body.String(), "auth>priv"; got != want {
		t.Errorf("private mw: want %q, got %q", want, got)
	}
}

func TestRouterMountStripsPrefix(t *testing.T) {
	// A mounted sub-handler should see paths relative to its mount point.
	sub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("sub:" + r.URL.Path))
	})

	r := NewRouter().(*router)
	r.Mount("/admin", sub)

	mux := http.NewServeMux()
	r.build(mux, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/things", nil))
	if got, want := rec.Body.String(), "sub:/things"; got != want {
		t.Fatalf("mount strip: want %q, got %q", want, got)
	}
}

func TestRouterMethodMismatchReturns405(t *testing.T) {
	// Go 1.22+ stdlib mux returns 405 for method mismatches when a
	// matching pattern exists for another method. Confirms we're not
	// silently 404-ing.
	r := NewRouter().(*router)
	r.Get("/only-get", stubHandler("g"))

	mux := http.NewServeMux()
	r.build(mux, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/only-get", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", rec.Code)
	}
}

func TestRouterUnknownPathReturns404(t *testing.T) {
	r := NewRouter().(*router)
	r.Get("/known", stubHandler("k"))

	mux := http.NewServeMux()
	r.build(mux, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/unknown", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}

func TestRouterPathValueFlowsThroughStdlib(t *testing.T) {
	r := NewRouter().(*router)
	r.Route("/v1/workspaces/{wsid}", func(r Router) {
		r.Get("/members/{memberid}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(r.PathValue("wsid") + "/" + r.PathValue("memberid")))
		}))
	})

	mux := http.NewServeMux()
	r.build(mux, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/workspaces/W1/members/M9", nil))
	if got, want := rec.Body.String(), "W1/M9"; got != want {
		t.Fatalf("path values: want %q, got %q", want, got)
	}
}

func TestJoinPath(t *testing.T) {
	cases := []struct {
		prefix, suffix, want string
	}{
		{"", "", ""},
		{"", "/x", "/x"},
		{"", "x", "/x"},
		{"/v1", "", "/v1"},
		{"/v1", "/users", "/v1/users"},
		{"/v1/", "/users", "/v1/users"},
		{"/v1", "users", "/v1/users"},
		{"/v1/", "users", "/v1/users"},
		{"/v1/users", "/{id}", "/v1/users/{id}"},
	}
	for _, c := range cases {
		got := joinPath(c.prefix, c.suffix)
		if got != c.want {
			t.Errorf("joinPath(%q, %q): want %q, got %q", c.prefix, c.suffix, c.want, got)
		}
	}
}

// TestRouterWithAndNestedRouteCompose verifies the harder case:
// With(mw1).Route("/foo", func(r){ r.Get("/bar", h) }) should apply
// mw1 to GET /foo/bar.
func TestRouterWithAndNestedRouteCompose(t *testing.T) {
	r := NewRouter().(*router)
	r.With(recordingMW("auth")).Route("/v1", func(r Router) {
		r.Get("/x", stubHandler("h"))
	})

	mux := http.NewServeMux()
	r.build(mux, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/x", nil))
	if got, want := rec.Body.String(), "auth>h"; got != want {
		t.Fatalf("With+Route mw: want %q, got %q", want, got)
	}
}

func TestRouterEmptyLeafPathMeansBarePrefix(t *testing.T) {
	// r.Post("", h) inside Route("/x", ...) registers POST /x exactly,
	// not POST /x/. Convenient for the "create on the collection"
	// shape.
	r := NewRouter().(*router)
	r.Route("/users", func(r Router) {
		r.Post("", stubHandler("create"))
	})

	mux := http.NewServeMux()
	r.build(mux, nil)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users", nil))
	if got, want := strings.TrimSpace(rec.Body.String()), "create"; got != want {
		t.Fatalf("bare-prefix POST: want %q, got %q (code=%d)", want, got, rec.Code)
	}
}
