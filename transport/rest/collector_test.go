package rest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/fromforgesoftware/go-kit/openapi"
	kitrest "github.com/fromforgesoftware/go-kit/transport/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// End-to-end test for OpenAPI spec generation via the kit's server +
// router + collector pipeline. Uses lightweight, package-local request
// and response types so we don't need to satisfy every kit generic
// (the handler factories themselves are covered by their own tests).

type widgetReq struct {
	Name string `json:"name"`
}

type widgetResp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func plainCreateHandler() http.Handler {
	endpoint := func(_ context.Context, in widgetReq) (widgetResp, error) {
		return widgetResp{ID: "w1", Name: in.Name}, nil
	}
	dec := kitrest.NewHTTPDecoder(func(_ context.Context, r *http.Request) (widgetReq, error) {
		var req widgetReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		return req, nil
	})
	enc := kitrest.RestJSONEncoder(func(o widgetResp) widgetResp { return o }, http.StatusCreated)
	return kitrest.NewHandler(endpoint, dec, enc,
		kitrest.HandlerAllowsEmptyReq(true),
		kitrest.HandlerWithSuccessStatus(http.StatusCreated),
	)
}

// jsonapiLikeHandler is a hand-rolled handler that mimics what the
// JsonApi factories produce — its SetupOperation emits a JSON:API
// envelope on the response (using jsonapi.Document[widgetResp]). Avoids
// satisfying every generic in NewJsonApiCreateHandler while still
// exercising the same docs-override path.
func jsonapiLikeHandler(opts ...kitrest.HandlerOpt) http.Handler {
	endpoint := func(_ context.Context, in widgetReq) (widgetResp, error) {
		return widgetResp{ID: "w1", Name: in.Name}, nil
	}
	dec := kitrest.NewHTTPDecoder(func(_ context.Context, r *http.Request) (widgetReq, error) {
		var req widgetReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		return req, nil
	})
	enc := kitrest.RestJSONEncoder(func(o widgetResp) widgetResp { return o }, http.StatusCreated)
	// Emulate what NewJsonApiCreateHandler does: install an envelope
	// docs override. Since handlerWithDocsOverride is unexported, we
	// use HandlerWithOpenAPI to override the schema via Raw.
	envelope := kitrest.HandlerWithOpenAPI(openapi.Raw(func(oc openapi.OperationContext) error {
		var doc jsonapi.Document[widgetResp]
		oc.AddRequest(doc)
		oc.AddResponse(http.StatusCreated, doc)
		return nil
	}))
	return kitrest.NewHandler(endpoint, dec, enc, append([]kitrest.HandlerOpt{
		kitrest.HandlerAllowsEmptyReq(true),
		kitrest.HandlerWithSuccessStatus(http.StatusCreated),
		envelope,
	}, opts...)...)
}

type ctrl struct{}

func (ctrl) Routes(r kitrest.Router) {
	// Plain JSON handler — no envelope, no annotations.
	r.Post("/widgets", plainCreateHandler())

	// JSON:API envelope, with annotations.
	r.Post("/users", jsonapiLikeHandler(
		kitrest.HandlerWithOpenAPI(
			openapi.Summary("Create a user"),
			openapi.Tags("users"),
			openapi.Errors(http.StatusUnauthorized, http.StatusConflict),
		),
	))

	// Public route — explicitly no security.
	r.Post("/login", jsonapiLikeHandler(
		kitrest.HandlerWithOpenAPI(
			openapi.Tags("auth"),
			openapi.NoSecurity(),
		),
	))
}

func TestServer_OpenAPISpecExposed(t *testing.T) {
	srv := buildServer(t)
	body := fetchSpec(t, srv.Handler)
	assert.Equal(t, "3.1.0", body["openapi"])
	info := mapAt(body, "info")
	assert.Equal(t, "Forge Test", info["title"])
	assert.Equal(t, "0.0.1", info["version"])
}

func TestServer_SecuritySchemesRegistered(t *testing.T) {
	srv := buildServer(t)
	body := fetchSpec(t, srv.Handler)

	schemes := mapAt(body, "components", "securitySchemes")
	require.Contains(t, schemes, "bearerAuth")
	bearer := schemes["bearerAuth"].(map[string]any)
	assert.Equal(t, "http", bearer["type"])
	assert.Equal(t, "bearer", bearer["scheme"])
	assert.Equal(t, "JWT", bearer["bearerFormat"])

	// Root-level default security.
	root := body["security"].([]any)
	require.Len(t, root, 1)
	assert.Contains(t, root[0].(map[string]any), "bearerAuth")
}

func TestServer_PlainHandlerSchemaIsBareDTO(t *testing.T) {
	srv := buildServer(t)
	body := fetchSpec(t, srv.Handler)

	schema := resolveSchema(body, mapAt(body, "paths", "/widgets", "post", "requestBody", "content", "application/json", "schema"))
	require.NotNil(t, schema, "plain handler must declare a request body schema")
	props := mapAt(schema, "properties")
	require.NotNil(t, props)
	// Bare widgetReq → has a "name" property at the top level (NOT nested under data.attributes).
	require.Contains(t, props, "name")
	require.NotContains(t, props, "data")
}

func TestServer_JsonApiHandlerSchemaIsEnvelope(t *testing.T) {
	srv := buildServer(t)
	body := fetchSpec(t, srv.Handler)

	schema := resolveSchema(body, mapAt(body, "paths", "/users", "post", "requestBody", "content", "application/json", "schema"))
	require.NotNil(t, schema, "JSON:API handler must declare a request body schema")
	props := mapAt(schema, "properties")
	require.NotNil(t, props)
	require.Contains(t, props, "data", "JSON:API envelope must wrap content under 'data'")
}

func TestServer_HandlerWithOpenAPIAnnotations(t *testing.T) {
	srv := buildServer(t)
	body := fetchSpec(t, srv.Handler)

	post := mapAt(body, "paths", "/users", "post")
	assert.Equal(t, "Create a user", post["summary"])
	assert.Equal(t, []any{"users"}, post["tags"])

	responses := mapAt(post, "responses")
	require.Contains(t, responses, "401", "DocErrors(401, ...) must produce a 401 response entry")
	require.Contains(t, responses, "409", "DocErrors(..., 409) must produce a 409 response entry")
}

func TestServer_NoSecurityOverridesDefault(t *testing.T) {
	srv := buildServer(t)
	body := fetchSpec(t, srv.Handler)

	loginSec, ok := mapAt(body, "paths", "/login", "post")["security"]
	require.True(t, ok, "/login post must override default security")
	assert.Equal(t, []any{map[string]any{}}, loginSec,
		"NoSecurity() should emit security: [{}] which spec consumers treat as 'no auth'")

	// /widgets post inherits root default — has no security override.
	_, hasOverride := mapAt(body, "paths", "/widgets", "post")["security"]
	assert.False(t, hasOverride)
}

func TestServer_UnannotatedRouteHasNoAutoErrors(t *testing.T) {
	srv := buildServer(t)
	body := fetchSpec(t, srv.Handler)

	responses := mapAt(body, "paths", "/widgets", "post", "responses")
	for code := range responses {
		assert.Contains(t, []string{"201"}, code,
			"unannotated plain handler should only declare its success response, got %q", code)
	}
}

// --- helpers ------------------------------------------------------------

func buildServer(t *testing.T) *http.Server {
	t.Helper()
	srv := kitrest.NewServer(
		kitrest.WithControllers(ctrl{}),
		kitrest.WithOpenAPI(
			openapi.SpecTitle("Forge Test"),
			openapi.SpecVersion("0.0.1"),
			openapi.SpecSecurityScheme("bearerAuth", openapi.BearerJWT()),
			openapi.DefaultSecurity("bearerAuth"),
		),
	)
	require.NotNil(t, srv)
	return srv
}

func fetchSpec(t *testing.T, h http.Handler) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func mapAt(m map[string]any, keys ...string) map[string]any {
	cur := m
	for _, k := range keys {
		next, ok := cur[k].(map[string]any)
		if !ok {
			return nil
		}
		cur = next
	}
	return cur
}

func resolveSchema(spec map[string]any, schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	if ref, ok := schema["$ref"].(string); ok {
		const prefix = "#/components/schemas/"
		if len(ref) > len(prefix) && ref[:len(prefix)] == prefix {
			name := ref[len(prefix):]
			return mapAt(spec, "components", "schemas", name)
		}
	}
	return schema
}
