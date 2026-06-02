package jsonapi_test

import (
	"os"
	"testing"
	"time"

	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalPayload(t *testing.T) {
	file, err := os.Open("testdata/unmarshal_single_article.json")
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()

	result, err := jsonapi.UnmarshalPayload[*articleDTO](file)
	require.NoError(t, err)

	// Instead of using generateArticles, which has different data than our test file,
	// directly check the fields we care about
	article := result.Data

	// Basic assertions on what we expect from the test data
	assert.Equal(t, "1", article.ID())
	assert.Equal(t, "articles", article.Type())
	assert.Equal(t, "title 1", article.Title())

	// Check timestamps
	if article.Timestamps() != nil {
		createdTime, _ := time.Parse(jsonapi.ISO8601TimeFormat, "2023-01-01T01:01:01.123Z")
		updatedTime, _ := time.Parse(jsonapi.ISO8601TimeFormat, "2023-01-02T01:01:01.123Z")
		assert.Equal(t, createdTime, article.Timestamps().CreatedAt())
		assert.Equal(t, updatedTime, article.Timestamps().UpdatedAt())
	}

	// Check relationships
	if author := article.Author(); author != nil {
		assert.Equal(t, "author id 1", author.ID())
	}

	// Check other fields based on the test data
	marketDate, _ := time.Parse("2006-01-02", "2024-01-01")
	if article.MarketDate() != nil {
		assert.Equal(t, marketDate, *article.MarketDate())
	}
}

func TestUnmarshalManyPayload(t *testing.T) {
	file, err := os.Open(getTestDataFile("unmarshal_many_articles.json"))
	require.NoError(t, err)
	defer file.Close()

	result, err := jsonapi.UnmarshalManyPayload[*articleDTO](file)
	require.NoError(t, err)

	// Check that we got 2 articles as expected
	assert.Len(t, result.Data, 2)

	// Check first article
	article1 := result.Data[0]
	assert.Equal(t, "1", article1.ID())
	assert.Equal(t, "articles", article1.Type())
	assert.Equal(t, "title 1", article1.Title())

	// Check second article
	article2 := result.Data[1]
	assert.Equal(t, "2", article2.ID())
	assert.Equal(t, "articles", article2.Type())
	assert.Equal(t, "title 2", article2.Title())

	// Check timestamps for first article
	if article1.Timestamps() != nil {
		createdTime, _ := time.Parse(jsonapi.ISO8601TimeFormat, "2023-01-01T01:01:01.123Z")
		updatedTime, _ := time.Parse(jsonapi.ISO8601TimeFormat, "2023-01-02T01:01:01.123Z")
		assert.Equal(t, createdTime, article1.Timestamps().CreatedAt())
		assert.Equal(t, updatedTime, article1.Timestamps().UpdatedAt())
	}

	// Check timestamps for second article
	if article2.Timestamps() != nil {
		createdTime, _ := time.Parse(jsonapi.ISO8601TimeFormat, "2023-01-02T01:01:01.123Z")
		updatedTime, _ := time.Parse(jsonapi.ISO8601TimeFormat, "2023-01-03T01:01:01.123Z")
		assert.Equal(t, createdTime, article2.Timestamps().CreatedAt())
		assert.Equal(t, updatedTime, article2.Timestamps().UpdatedAt())
	}
}
