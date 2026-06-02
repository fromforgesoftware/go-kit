package openapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fromforgesoftware/go-kit/openapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type petCreateReq struct {
	Name string `json:"name"`
}

type petResp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// fakeHandler is an http.Handler that also implements openapi.Preparer
// so we can drive the collector without standing up a real kit handler.
type fakeHandler struct {
	setup func(oc openapi.OperationContext) error
}

func (h fakeHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}
func (h fakeHandler) SetupOperation(oc openapi.OperationContext) error {
	if h.setup == nil {
		return nil
	}
	return h.setup(oc)
}

func TestCollector_BasicSpec(t *testing.T) {
	c, err := openapi.NewCollector(openapi.NewReflector(), openapi.SpecConfig{
		Title:   "Test API",
		Version: "1.2.3",
	})
	require.NoError(t, err)

	h := fakeHandler{setup: func(oc openapi.OperationContext) error {
		oc.AddRequest(petCreateReq{})
		oc.AddResponse(http.StatusCreated, petResp{})
		oc.SetSummary("Create a pet")
		oc.SetTags("pets")
		return nil
	}}

	require.NoError(t, c.CollectOperation(http.MethodPost, "/pets", h))

	body := fetchSpec(t, c)
	assert.Equal(t, "3.1.0", str(body, "openapi"))
	assert.Equal(t, "Test API", strPath(body, "info", "title"))
	assert.Equal(t, "1.2.3", strPath(body, "info", "version"))

	post := mapPath(body, "paths", "/pets", "post")
	assert.Equal(t, "Create a pet", post["summary"])
	tags := post["tags"].([]any)
	assert.Equal(t, []any{"pets"}, tags)

	resp201 := mapPath(post, "responses", "201")
	assert.NotNil(t, resp201)
}

func TestCollector_SecuritySchemesAndDefault(t *testing.T) {
	c, err := openapi.NewCollector(openapi.NewReflector(), openapi.SpecConfig{
		Title:   "Secured",
		Version: "0.1.0",
		SecuritySchemes: map[string]openapi.SecurityScheme{
			"bearerAuth": openapi.BearerJWT(),
			"apiKey":     openapi.APIKeyHeader("X-API-Key"),
		},
		DefaultSecurity: []string{"bearerAuth"},
	})
	require.NoError(t, err)

	body := fetchSpec(t, c)
	schemes := mapPath(body, "components", "securitySchemes")
	require.Contains(t, schemes, "bearerAuth")
	require.Contains(t, schemes, "apiKey")

	bearer := schemes["bearerAuth"].(map[string]any)
	assert.Equal(t, "http", bearer["type"])
	assert.Equal(t, "bearer", bearer["scheme"])
	assert.Equal(t, "JWT", bearer["bearerFormat"])

	apiKey := schemes["apiKey"].(map[string]any)
	assert.Equal(t, "apiKey", apiKey["type"])
	assert.Equal(t, "header", apiKey["in"])
	assert.Equal(t, "X-API-Key", apiKey["name"])

	root := body["security"].([]any)
	require.Len(t, root, 1)
	assert.Contains(t, root[0].(map[string]any), "bearerAuth")
}

func TestCollector_PerOperationSecurity(t *testing.T) {
	c, err := openapi.NewCollector(openapi.NewReflector(), openapi.SpecConfig{
		Title:   "Per-op security",
		Version: "0.1.0",
		SecuritySchemes: map[string]openapi.SecurityScheme{
			"bearerAuth": openapi.BearerJWT(),
		},
		DefaultSecurity: []string{"bearerAuth"},
	})
	require.NoError(t, err)

	// One route explicitly says "no security" — should override default.
	publicHandler := fakeHandler{setup: func(oc openapi.OperationContext) error {
		oc.AddRequest(petCreateReq{})
		oc.AddResponse(http.StatusOK, petResp{})
		oc.SetSecurity() // explicit empty = public
		return nil
	}}
	require.NoError(t, c.CollectOperation(http.MethodPost, "/login", publicHandler))

	// One route uses the default (no SetSecurity call).
	authedHandler := fakeHandler{setup: func(oc openapi.OperationContext) error {
		oc.AddRequest(petCreateReq{})
		oc.AddResponse(http.StatusCreated, petResp{})
		return nil
	}}
	require.NoError(t, c.CollectOperation(http.MethodPost, "/pets", authedHandler))

	body := fetchSpec(t, c)

	loginSec, ok := mapPath(body, "paths", "/login", "post")["security"]
	if assert.True(t, ok, "/login post must have explicit security field") {
		// Materialised as [{}] (a single empty requirement) so that
		// the omitempty json tag doesn't drop the override. Spec
		// consumers treat this identically to security: [].
		assert.Equal(t, []any{map[string]any{}}, loginSec)
	}

	// /pets POST inherits root security (no per-op override).
	_, hasOverride := mapPath(body, "paths", "/pets", "post")["security"]
	assert.False(t, hasOverride, "/pets post should inherit root security, not declare its own")
}

func TestCollector_TagsAndServers(t *testing.T) {
	c, err := openapi.NewCollector(openapi.NewReflector(), openapi.SpecConfig{
		Title:   "Tagged",
		Version: "0.1.0",
		Tags: []openapi.SpecTagInfo{
			{Name: "pets", Description: "Pet CRUD."},
			{Name: "auth", Description: "Auth flows."},
		},
		Servers: []openapi.SpecServer{
			{URL: "https://api.example.com", Description: "Production"},
		},
	})
	require.NoError(t, err)

	body := fetchSpec(t, c)
	tags := body["tags"].([]any)
	require.Len(t, tags, 2)
	assert.Equal(t, "pets", tags[0].(map[string]any)["name"])
	assert.Equal(t, "Pet CRUD.", tags[0].(map[string]any)["description"])

	servers := body["servers"].([]any)
	require.Len(t, servers, 1)
	assert.Equal(t, "https://api.example.com", servers[0].(map[string]any)["url"])
}

// fetchSpec serves the spec via http and returns the decoded JSON.
func fetchSpec(t *testing.T, c *openapi.Collector) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	c.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func str(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return v
}

func strPath(m map[string]any, keys ...string) string {
	cur := m
	for i, k := range keys {
		if i == len(keys)-1 {
			return str(cur, k)
		}
		next, ok := cur[k].(map[string]any)
		if !ok {
			return ""
		}
		cur = next
	}
	return ""
}

func mapPath(m map[string]any, keys ...string) map[string]any {
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
