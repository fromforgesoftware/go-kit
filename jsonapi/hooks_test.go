package jsonapi_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fromforgesoftware/go-kit/jsonapi"
	"github.com/stretchr/testify/assert"
)

// A simple struct for testing the hooks
type testHookArticle struct {
	ID       string `jsonapi:"primary"`
	Type     string `jsonapi:"type"`
	Title    string `jsonapi:"attr,title"`
	Content  string `jsonapi:"attr,content"`
	Password string `jsonapi:"attr,password,omitempty"`
}

// BeforeMarshal implements the MarshalHook interface
func (t *testHookArticle) BeforeMarshal() error {
	// Set default title if empty
	if t.Title == "" {
		t.Title = "Default Title"
	}

	// Mask the password
	if t.Password != "" {
		firstChar := string(t.Password[0])
		maskedPortion := strings.Repeat("*", len(t.Password)-1)
		t.Password = firstChar + maskedPortion
	}

	return nil
}

// AfterUnmarshal implements the UnmarshalHook interface
func (t *testHookArticle) AfterUnmarshal() error {
	// Add secured note to password
	if t.Password != "" && !strings.HasSuffix(t.Password, " (secured)") {
		t.Password = t.Password + " (secured)"
	}

	return nil
}

func TestHooks(t *testing.T) {
	// Create a test article with a password
	article := &testHookArticle{
		ID:       "1",
		Type:     "articles",
		Title:    "",
		Content:  "Test content",
		Password: "secret123",
	}

	// Marshal to JSON
	var buf bytes.Buffer
	err := jsonapi.MarshalPayload(&buf, article)
	assert.NoError(t, err)

	// Verify BeforeMarshal was called
	assert.Equal(t, "Default Title", article.Title)
	assert.True(t, strings.HasPrefix(article.Password, "s"), "Password should start with 's'")
	assert.NotEqual(t, "secret123", article.Password, "Password should be masked")

	// JSON data should contain masked password and not original password
	jsonData := buf.String()
	assert.NotContains(t, jsonData, "secret123")

	// Prepare for unmarshaling

	// Reset buffer and marshal again for unmarshaling
	buf.Reset()
	err = jsonapi.MarshalPayload(&buf, article)
	assert.NoError(t, err)

	// Unmarshal back
	result, err := jsonapi.UnmarshalPayload[*testHookArticle](&buf)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Data)

	// Verify AfterUnmarshal was called
	assert.True(t, strings.HasSuffix(result.Data.Password, " (secured)"),
		"Password should end with ' (secured)'")
}
