package testutil_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fromforgesoftware/go-kit/testutil"
)

func TestAssertStatus_PassesOnMatch(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.WriteHeader(http.StatusCreated)
	testutil.AssertStatus(t, rec, http.StatusCreated)
}

func TestAssertJSONBody_NormalisesShape(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/json")
	body, err := json.Marshal(map[string]any{"name": "Rex", "tag": "dog"})
	require.NoError(t, err)
	_, _ = rec.Write(body)

	type Pet struct {
		Name string `json:"name"`
		Tag  string `json:"tag"`
	}
	testutil.AssertJSONBody(t, rec, Pet{Name: "Rex", Tag: "dog"})
}

func TestDecodeJSON_Roundtrip(t *testing.T) {
	rec := httptest.NewRecorder()
	_, _ = rec.Write([]byte(`{"answer":42}`))

	type Answer struct {
		Answer int `json:"answer"`
	}
	got := testutil.DecodeJSON[Answer](t, rec)
	assert.Equal(t, 42, got.Answer)
}

func TestNewJSONRequest_SetsContentType(t *testing.T) {
	type Cmd struct {
		Greeting string `json:"greeting"`
	}
	req := testutil.NewJSONRequest(t, http.MethodPost, "/hi", Cmd{Greeting: "hi"})
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, http.MethodPost, req.Method)

	var got Cmd
	require.NoError(t, json.NewDecoder(req.Body).Decode(&got))
	assert.Equal(t, "hi", got.Greeting)
}
