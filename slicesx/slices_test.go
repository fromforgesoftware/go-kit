package slicesx_test

import (
	"fmt"
	"testing"

	"github.com/fromforgesoftware/go-kit/slicesx"
	"github.com/stretchr/testify/assert"
)

func TestMap(t *testing.T) {
	t.Run("transforms integers", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5}
		result := slicesx.Map(input, func(n int) int { return n * 2 })

		expected := []int{2, 4, 6, 8, 10}
		assert.Equal(t, expected, result)
	})

	t.Run("converts types", func(t *testing.T) {
		input := []int{1, 2, 3}
		result := slicesx.Map(input, func(n int) string {
			return fmt.Sprintf("%d", n)
		})

		expected := []string{"1", "2", "3"}
		assert.Equal(t, expected, result)
	})

	t.Run("handles empty slice", func(t *testing.T) {
		var input []int
		result := slicesx.Map(input, func(n int) int { return n * 2 })

		assert.Empty(t, result)
		assert.NotNil(t, result) // Should return empty slice, not nil
	})

	t.Run("handles single element", func(t *testing.T) {
		input := []int{42}
		result := slicesx.Map(input, func(n int) int { return n * 2 })

		expected := []int{84}
		assert.Equal(t, expected, result)
	})

	t.Run("extracts struct fields", func(t *testing.T) {
		type User struct {
			ID   string
			Name string
		}

		users := []User{
			{ID: "1", Name: "Alice"},
			{ID: "2", Name: "Bob"},
			{ID: "3", Name: "Charlie"},
		}

		ids := slicesx.Map(users, func(u User) string { return u.ID })
		names := slicesx.Map(users, func(u User) string { return u.Name })

		assert.Equal(t, []string{"1", "2", "3"}, ids)
		assert.Equal(t, []string{"Alice", "Bob", "Charlie"}, names)
	})

	t.Run("handles pointers", func(t *testing.T) {
		input := []int{1, 2, 3}
		result := slicesx.Map(input, func(n int) *int {
			val := n * 2
			return &val
		})

		assert.Len(t, result, 3)
		assert.Equal(t, 2, *result[0])
		assert.Equal(t, 4, *result[1])
		assert.Equal(t, 6, *result[2])
	})
}

func TestFind(t *testing.T) {
	t.Run("finds existing element", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5}
		result, found := slicesx.Find(input, func(n int) bool { return n > 3 })

		assert.True(t, found)
		assert.Equal(t, 4, result)
	})

	t.Run("returns first match", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5}
		result, found := slicesx.Find(input, func(n int) bool { return n > 2 })

		assert.True(t, found)
		assert.Equal(t, 3, result) // First element > 2
	})

	t.Run("returns false when not found", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5}
		result, found := slicesx.Find(input, func(n int) bool { return n > 10 })

		assert.False(t, found)
		assert.Equal(t, 0, result) // Zero value for int
	})

	t.Run("handles empty slice", func(t *testing.T) {
		var input []int
		result, found := slicesx.Find(input, func(n int) bool { return true })

		assert.False(t, found)
		assert.Equal(t, 0, result)
	})

	t.Run("finds in struct slice", func(t *testing.T) {
		type User struct {
			ID     string
			Active bool
		}

		users := []User{
			{ID: "1", Active: false},
			{ID: "2", Active: true},
			{ID: "3", Active: true},
		}

		result, found := slicesx.Find(users, func(u User) bool { return u.Active })

		assert.True(t, found)
		assert.Equal(t, "2", result.ID)
	})

	t.Run("finds string by condition", func(t *testing.T) {
		names := []string{"Alice", "Bob", "Charlie", "David"}
		result, found := slicesx.Find(names, func(s string) bool {
			return len(s) > 5
		})

		assert.True(t, found)
		assert.Equal(t, "Charlie", result)
	})

	t.Run("returns zero value for string when not found", func(t *testing.T) {
		names := []string{"Al", "Bo"}
		result, found := slicesx.Find(names, func(s string) bool {
			return len(s) > 5
		})

		assert.False(t, found)
		assert.Equal(t, "", result) // Zero value for string
	})
}

// BenchmarkMap benchmarks the Map function with various slice sizes.
func BenchmarkMap(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			input := make([]int, size)
			for i := range input {
				input[i] = i
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = slicesx.Map(input, func(n int) int { return n * 2 })
			}
		})
	}
}

// BenchmarkMapTypeConversion benchmarks Map with type conversion.
func BenchmarkMapTypeConversion(b *testing.B) {
	input := make([]int, 1000)
	for i := range input {
		input[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = slicesx.Map(input, func(n int) string {
			return fmt.Sprintf("%d", n)
		})
	}
}

// BenchmarkFind benchmarks Find function with different scenarios.
func BenchmarkFind(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("first_element_size_%d", size), func(b *testing.B) {
			input := make([]int, size)
			for i := range input {
				input[i] = i
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = slicesx.Find(input, func(n int) bool { return n == 0 })
			}
		})

		b.Run(fmt.Sprintf("middle_element_size_%d", size), func(b *testing.B) {
			input := make([]int, size)
			for i := range input {
				input[i] = i
			}
			target := size / 2

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = slicesx.Find(input, func(n int) bool { return n == target })
			}
		})

		b.Run(fmt.Sprintf("not_found_size_%d", size), func(b *testing.B) {
			input := make([]int, size)
			for i := range input {
				input[i] = i
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = slicesx.Find(input, func(n int) bool { return n > size })
			}
		})
	}
}

// BenchmarkMapVsLoop compares Map performance vs traditional loop.
func BenchmarkMapVsLoop(b *testing.B) {
	input := make([]int, 1000)
	for i := range input {
		input[i] = i
	}

	b.Run("using_Map", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = slicesx.Map(input, func(n int) int { return n * 2 })
		}
	})

	b.Run("using_loop", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			output := make([]int, len(input))
			for j, v := range input {
				output[j] = v * 2
			}
			_ = output
		}
	})
}
