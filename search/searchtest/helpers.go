package searchtest

import (
	"testing"

	"github.com/fromforgesoftware/go-kit/search"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

func OptsEqual(t *testing.T, expected, got []search.Option) {
	t.Helper()

	assert.True(t, cmp.Equal(expected, got))
}

func OptsDiff(t *testing.T, expected, got []search.Option) {
	t.Helper()

	assert.False(t, cmp.Equal(expected, got))
}
