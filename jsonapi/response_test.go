package jsonapi_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/fromforgesoftware/go-kit/jsonapi/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalPayload_WithMeta(t *testing.T) {
	article := articlesToDTO(generateArticles(1)...)[0]

	var buf bytes.Buffer
	meta := jsonapi.Meta{"valid": true, "count": float64(1)}
	require.NoError(t, jsonapi.MarshalPayload(&buf, article, jsonapi.WithMeta(&meta)))

	var doc struct {
		Data struct {
			Meta map[string]any `json:"meta"`
		} `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))

	assert.Equal(t, true, doc.Meta["valid"], "document-level meta must be set")
	assert.Equal(t, float64(1), doc.Meta["count"])
	_, leaked := doc.Data.Meta["valid"]
	assert.False(t, leaked, "WithMeta must land at the document root only, not on the resource")
}

func TestMarshalPayload(t *testing.T) {
	// Create article using constructor with sample article
	article := articlesToDTO(generateArticles(1)...)[0]

	// Marshal to JSON with specific relationships included
	var buf bytes.Buffer
	err := jsonapi.MarshalPayload(&buf, article, jsonapi.WithInclude("coauthors", "subarticles.subarticles"))
	assert.NoError(t, err)

	// Use golden file for comparison
	goldenPath := "testdata/marshal_single_article.golden"
	updateGolden := os.Getenv("UPDATE_GOLDEN") != ""
	helpers.AssertEqualFile(t, goldenPath, &buf, updateGolden)
}

func TestMarshalManyPayload(t *testing.T) {
	articles := articlesToDTO(generateArticles(2)...)

	listResponse := &listResponse[*articleDTO]{
		data:  articles,
		total: len(articles),
	}

	var buf bytes.Buffer
	err := jsonapi.MarshalManyPayloads(&buf, listResponse, jsonapi.WithInclude("author", "publishers"))
	require.NoError(t, err)

	// Use golden file for comparison
	goldenPath := "testdata/marshal_many_articles.golden"
	updateGolden := os.Getenv("UPDATE_GOLDEN") != ""
	helpers.AssertEqualGoldenFile(
		t, goldenPath, bytes.NewReader(buf.Bytes()), updateGolden,
	)
}
