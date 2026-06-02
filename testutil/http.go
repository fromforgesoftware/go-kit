package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertStatus checks the HTTP status without reading the body.
func AssertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	assert.Equalf(t, want, rec.Code, "status mismatch — body: %s", rec.Body.String())
}

// AssertJSONBody decodes the recorder's body into want's shape and
// asserts equality. The expected value can be a struct, map, or any
// JSON-compatible Go value.
func AssertJSONBody(t *testing.T, rec *httptest.ResponseRecorder, want any) {
	t.Helper()
	got := decodeJSON[any](t, rec.Body)
	wantNormalised := decodeJSON[any](t, mustJSON(t, want))
	assert.Equal(t, wantNormalised, got)
}

// DecodeJSON decodes the recorder's body into T. Useful when a test
// wants to inspect specific fields rather than match the full payload.
func DecodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	return decodeJSON[T](t, rec.Body)
}

func decodeJSON[T any](t *testing.T, r io.Reader) T {
	t.Helper()
	var v T
	require.NoError(t, json.NewDecoder(r).Decode(&v))
	return v
}

func mustJSON(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewReader(b)
}

// NewJSONRequest builds an *http.Request whose body is body marshalled
// as JSON with the matching Content-Type header. Saves three lines of
// boilerplate in transport tests.
func NewJSONRequest(t *testing.T, method, target string, body any) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, mustJSON(t, body))
	req.Header.Set("Content-Type", "application/json")
	return req
}
