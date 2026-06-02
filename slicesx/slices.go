package slicesx

// Map transforms each element of the input slice using the provided transform function.
//
// The function creates a new slice with the transformed elements, leaving the
// original slice unchanged. If the input slice is empty, an empty output slice
// is returned.
//
// Type Parameters:
//   - I: Input element type
//   - O: Output element type
//
// Example:
//
//	numbers := []int{1, 2, 3}
//	doubled := slicesx.Map(numbers, func(n int) int { return n * 2 })
//	// Result: []int{2, 4, 6}
//
//	// Convert types
//	strings := slicesx.Map(numbers, func(n int) string { return fmt.Sprintf("%d", n) })
//	// Result: []string{"1", "2", "3"}
func Map[I any, O any](input []I, transform func(I) O) []O {
	o := make([]O, len(input))
	if len(input) < 1 {
		return o
	}
	for i, e := range input {
		o[i] = transform(e)
	}
	return o
}

// Find returns the first element in the slice that satisfies the predicate function.
//
// The function iterates through the slice and returns the first element for which
// the predicate returns true, along with a boolean indicating whether a match was found.
// If no element matches, the zero value for the type and false are returned.
//
// Type Parameters:
//   - I: Element type
//
// Example:
//
//	numbers := []int{1, 2, 3, 4, 5}
//	result, found := slicesx.Find(numbers, func(n int) bool { return n > 3 })
//	// result = 4, found = true
//
//	result, found := slicesx.Find(numbers, func(n int) bool { return n > 10 })
//	// result = 0 (zero value), found = false
func Find[I any](input []I, predicate func(I) bool) (element I, found bool) {
	for _, e := range input {
		if predicate(e) {
			return e, true
		}
	}

	return element, false
}
