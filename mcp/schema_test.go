package mcp_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/mcp"
)

func decodeSchema(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func TestSchemaForPrimitives(t *testing.T) {
	type P struct {
		S string  `json:"s"`
		I int     `json:"i"`
		F float64 `json:"f"`
		B bool    `json:"b"`
	}
	got := decodeSchema(t, mcp.SchemaFor[P]())
	props := got["properties"].(map[string]any)
	assert.Equal(t, "string", props["s"].(map[string]any)["type"])
	assert.Equal(t, "integer", props["i"].(map[string]any)["type"])
	assert.Equal(t, "number", props["f"].(map[string]any)["type"])
	assert.Equal(t, "boolean", props["b"].(map[string]any)["type"])
	req := got["required"].([]any)
	assert.ElementsMatch(t, []any{"s", "i", "f", "b"}, req)
}

func TestSchemaOmitemptyMakesOptional(t *testing.T) {
	type P struct {
		Required string `json:"required"`
		Optional string `json:"optional,omitempty"`
	}
	got := decodeSchema(t, mcp.SchemaFor[P]())
	req, _ := got["required"].([]any)
	assert.Contains(t, req, "required")
	assert.NotContains(t, req, "optional")
}

func TestSchemaPointerIsOptional(t *testing.T) {
	type P struct {
		Maybe *string `json:"maybe"`
	}
	got := decodeSchema(t, mcp.SchemaFor[P]())
	_, hasReq := got["required"]
	assert.False(t, hasReq, "pointer fields should not be required")
}

func TestSchemaDescriptionTag(t *testing.T) {
	type P struct {
		N int `json:"n" desc:"the count"`
	}
	got := decodeSchema(t, mcp.SchemaFor[P]())
	props := got["properties"].(map[string]any)
	assert.Equal(t, "the count", props["n"].(map[string]any)["description"])
}

func TestSchemaSliceAndMap(t *testing.T) {
	type P struct {
		Names []string       `json:"names"`
		Meta  map[string]int `json:"meta,omitempty"`
	}
	got := decodeSchema(t, mcp.SchemaFor[P]())
	props := got["properties"].(map[string]any)
	names := props["names"].(map[string]any)
	assert.Equal(t, "array", names["type"])
	assert.Equal(t, "string", names["items"].(map[string]any)["type"])
	meta := props["meta"].(map[string]any)
	assert.Equal(t, "object", meta["type"])
	assert.Equal(t, "integer", meta["additionalProperties"].(map[string]any)["type"])
}

func TestSchemaNestedStruct(t *testing.T) {
	type Inner struct {
		X int `json:"x"`
	}
	type Outer struct {
		I Inner `json:"i"`
	}
	got := decodeSchema(t, mcp.SchemaFor[Outer]())
	props := got["properties"].(map[string]any)
	inner := props["i"].(map[string]any)
	assert.Equal(t, "object", inner["type"])
	assert.Contains(t, inner["properties"], "x")
}

func TestSchemaIgnoreDashTag(t *testing.T) {
	type P struct {
		Hidden string `json:"-"`
		Shown  string `json:"shown"`
	}
	got := decodeSchema(t, mcp.SchemaFor[P]())
	props := got["properties"].(map[string]any)
	assert.NotContains(t, props, "Hidden")
	assert.NotContains(t, props, "-")
	assert.Contains(t, props, "shown")
}

func TestSchemaForAnyEmitsObject(t *testing.T) {
	got := decodeSchema(t, mcp.SchemaFor[any]())
	assert.Equal(t, "object", got["type"])
}
