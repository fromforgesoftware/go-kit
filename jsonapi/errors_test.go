package jsonapi_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/fromforgesoftware/go-kit/jsonapi/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalErrors(t *testing.T) {
	// Create error objects to marshal
	errorObjects := []*jsonapi.ErrorObject{
		{
			ID:     "error-1",
			Title:  "Resource Not Found",
			Detail: "The requested resource could not be found",
			Status: "404",
			Code:   "resource_not_found",
			Source: &jsonapi.ErrorSource{
				Pointer: "/data/attributes/title",
			},
			Meta: &map[string]interface{}{
				"timestamp": "2023-01-01T12:00:00Z",
			},
		},
		{
			ID:     "error-2",
			Title:  "Invalid Attribute",
			Detail: "The attribute format is invalid",
			Status: "422",
			Code:   "invalid_attribute",
			Source: &jsonapi.ErrorSource{
				Parameter: "filter[name]",
			},
		},
	}

	// Marshal to JSON
	var buf bytes.Buffer
	err := jsonapi.MarshalErrors(&buf, errorObjects)
	assert.NoError(t, err)

	// Use golden file for comparison
	goldenPath := "testdata/marshal_error.golden"
	updateGolden := os.Getenv("UPDATE_GOLDEN") != ""
	helpers.AssertEqualFile(t, goldenPath, &buf, updateGolden)
}

func TestUnmarshalError(t *testing.T) {
	// Open test file with error response instead of a resource
	f, err := os.Open("testdata/unmarshal_error.json")
	require.NoError(t, err)
	defer f.Close()

	// Try to unmarshal the error response as a resource, which should fail
	_, err = jsonapi.UnmarshalPayload[*articleDTO](f)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "data is not a jsonapi representation")
}

func TestUnmarshalErrorsPayload(t *testing.T) {
	// Open test file containing JSON:API errors payload
	f, err := os.Open("testdata/unmarshal_error.json")
	require.NoError(t, err)
	defer f.Close()

	// Unmarshal the errors payload
	errorObjects, err := jsonapi.UnmarshalErrors(f)
	require.NoError(t, err)

	// Verify the parsed error objects
	assert.Len(t, errorObjects, 1)
	assert.Equal(t, "Missing Required Field", errorObjects[0].Title)
	assert.Equal(t, "This field is required", errorObjects[0].Detail)
	assert.Equal(t, "400", errorObjects[0].Status)
	assert.Equal(t, "MISSING_FIELD", errorObjects[0].Code)
	assert.Equal(t, "/data/attributes/name", errorObjects[0].Source.Pointer)

	// Check meta information
	meta := *errorObjects[0].Meta
	assert.Equal(t, "name", meta["field"])
}
