// Package helpers bundles JSON:API test utilities — diff assertions
// against a wire payload, matchers against decoded resources, golden-file
// comparison — for use from service-level integration tests.
package helpers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func AssertEqualTimeOnly(t *testing.T, expected, actual time.Time) {
	t.Helper()

	assert.Equal(t, expected.Hour(), actual.Hour())
	assert.Equal(t, expected.Minute(), actual.Minute())
	assert.Equal(t, expected.Second(), actual.Second())
}

func AssertEqualDateOnly(t *testing.T, expected, actual time.Time) {
	t.Helper()

	assert.Equal(t, expected.Year(), actual.Year())
	assert.Equal(t, expected.Month(), actual.Month())
	assert.Equal(t, expected.Day(), actual.Day())
}

func AssertEqualNullableDateOnly(t *testing.T, expected, actual *time.Time) {
	t.Helper()

	if expected == nil {
		assert.Nil(t, actual)
		return
	}

	AssertEqualDateOnly(t, *expected, *actual)
}
