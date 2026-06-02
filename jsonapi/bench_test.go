package jsonapi_test

import (
	"bytes"
	"testing"

	"github.com/fromforgesoftware/go-kit/jsonapi"
)

// petBench is a minimal JSON:API DTO. The point of the benchmark
// is to measure the reflection-based marshal path against a stable
// representative shape — primary id + type + a handful of attrs.
type petBench struct {
	ID     string `jsonapi:"primary"`
	Type   string `jsonapi:"type"`
	Name   string `jsonapi:"attr,name"`
	Tag    string `jsonapi:"attr,tag,omitempty"`
	Status string `jsonapi:"attr,status"`
}

func newPetBench(id, name string) petBench {
	return petBench{ID: id, Type: "pets", Name: name, Tag: "dog", Status: "available"}
}

// BenchmarkMarshalPayload_Single measures the cost of encoding a
// single resource to a JSON:API top-level Document. Mirrors what
// every /v1/<resource>/{id} response does.
func BenchmarkMarshalPayload_Single(b *testing.B) {
	pet := newPetBench("pet-1", "Rex")
	var buf bytes.Buffer
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		if err := jsonapi.MarshalPayload(&buf, pet); err != nil {
			b.Fatal(err)
		}
	}
}

// UnmarshalPayload benchmark deferred: this kit's unmarshal does a
// type-name check against an embedded resource registration that
// needs more setup than a minimal benchmark warrants. Added in a
// follow-up once the registration pattern is documented in §19.
