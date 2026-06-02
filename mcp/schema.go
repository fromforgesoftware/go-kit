package mcp

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
)

// SchemaFor returns a JSON-Schema document describing the public
// fields of T (recursively). Field metadata is read from struct tags:
//
//	json:"name,omitempty" — sets the property name (and omitempty makes
//	                        the field optional).
//	desc:"…"            — human-readable description for the field.
//	required:"true"     — force-include in the schema's required list
//	                        regardless of omitempty.
//
// Unsupported field types fall back to `{"type":"object"}` so the
// schema is always well-formed.
func SchemaFor[T any]() json.RawMessage {
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil {
		// Untyped (e.g. T = any) — emit a permissive schema.
		return json.RawMessage(`{"type":"object"}`)
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	doc := schemaForType(t)
	raw, _ := json.Marshal(doc)
	return raw
}

// ErrSchemaUnsupported is returned by schema introspection helpers
// when a field's Go type can't be mapped to JSON Schema. The schema
// generator itself never returns this — it falls back to a generic
// object — but it's exported so callers can implement strict checks.
var ErrSchemaUnsupported = errors.New("mcp: type not mappable to JSON Schema")

func schemaForType(t reflect.Type) map[string]any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		return map[string]any{
			"type":  "array",
			"items": schemaForType(t.Elem()),
		}
	case reflect.Map:
		return map[string]any{
			"type":                 "object",
			"additionalProperties": schemaForType(t.Elem()),
		}
	case reflect.Struct:
		return schemaForStruct(t)
	default:
		return map[string]any{"type": "object"}
	}
}

func schemaForStruct(t reflect.Type) map[string]any {
	props := map[string]any{}
	var required []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, omitEmpty, skip := parseJSONTag(f)
		if skip {
			continue
		}
		fs := schemaForType(f.Type)
		if d := f.Tag.Get("desc"); d != "" {
			fs["description"] = d
		}
		props[name] = fs
		if f.Tag.Get("required") == "true" {
			required = append(required, name)
			continue
		}
		if !omitEmpty && f.Type.Kind() != reflect.Pointer {
			required = append(required, name)
		}
	}
	out := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

func parseJSONTag(f reflect.StructField) (name string, omitEmpty, skip bool) {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	if tag == "" {
		return f.Name, false, false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	if name == "" {
		name = f.Name
	}
	for _, p := range parts[1:] {
		if p == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty, false
}
