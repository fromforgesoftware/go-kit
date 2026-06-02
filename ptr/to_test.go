package ptr_test

import (
	"testing"

	"github.com/fromforgesoftware/go-kit/ptr"
	"github.com/stretchr/testify/assert"
)

func TestTo(t *testing.T) {
	tests := []struct {
		name     string
		input    *int
		expected int
	}{
		{
			name:     "returns value from non-nil pointer",
			input:    ptr.Of(42),
			expected: 42,
		},
		{
			name:     "returns zero value for nil pointer",
			input:    nil,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ptr.To(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToWithString(t *testing.T) {
	t.Run("non-nil string pointer", func(t *testing.T) {
		str := "hello"
		result := ptr.To(&str)
		assert.Equal(t, "hello", result)
	})

	t.Run("nil string pointer returns empty string", func(t *testing.T) {
		var strPtr *string
		result := ptr.To(strPtr)
		assert.Equal(t, "", result)
	})
}

func TestToWithBool(t *testing.T) {
	t.Run("non-nil bool pointer", func(t *testing.T) {
		b := true
		result := ptr.To(&b)
		assert.True(t, result)
	})

	t.Run("nil bool pointer returns false", func(t *testing.T) {
		var bPtr *bool
		result := ptr.To(bPtr)
		assert.False(t, result)
	})
}

func TestSliceTo(t *testing.T) {
	tests := []struct {
		name     string
		input    []*int
		expected []int
	}{
		{
			name:     "empty slice",
			input:    []*int{},
			expected: []int{},
		},
		{
			name:     "slice with values",
			input:    []*int{ptr.Of(1), ptr.Of(2), ptr.Of(3)},
			expected: []int{1, 2, 3},
		},
		{
			name:     "slice with nil values returns zeros",
			input:    []*int{ptr.Of(1), nil, ptr.Of(3)},
			expected: []int{1, 0, 3},
		},
		{
			name:     "slice with all nils",
			input:    []*int{nil, nil, nil},
			expected: []int{0, 0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ptr.SliceTo(tt.input...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSliceToWithStrings(t *testing.T) {
	t.Run("mixed nil and non-nil strings", func(t *testing.T) {
		s1 := "hello"
		s2 := "world"

		input := []*string{&s1, nil, &s2}
		result := ptr.SliceTo(input...)

		expected := []string{"hello", "", "world"}
		assert.Equal(t, expected, result)
	})
}
