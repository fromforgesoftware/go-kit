// Package slicesx provides generic utility functions for working with slices.
//
// This package offers functional programming utilities for slice operations
// using Go generics, avoiding the need to write repetitive boilerplate code.
//
// # Map Operations
//
// Transform each element of a slice:
//
//	numbers := []int{1, 2, 3, 4, 5}
//	doubled := slicesx.Map(numbers, func(n int) int { return n * 2 })
//	// Result: []int{2, 4, 6, 8, 10}
//
// Convert between types:
//
//	ages := []int{25, 30, 35}
//	strings := slicesx.Map(ages, func(age int) string {
//	    return fmt.Sprintf("%d years", age)
//	})
//	// Result: []string{"25 years", "30 years", "35 years"}
//
// Extract fields from structs:
//
//	users := []User{{ID: "1", Name: "Alice"}, {ID: "2", Name: "Bob"}}
//	ids := slicesx.Map(users, func(u User) string { return u.ID })
//	// Result: []string{"1", "2"}
//
// # Find Operations
//
// Find the first element matching a condition:
//
//	numbers := []int{1, 2, 3, 4, 5}
//	found, ok := slicesx.Find(numbers, func(n int) bool { return n > 3 })
//	// found = 4, ok = true
//
//	notFound, ok := slicesx.Find(numbers, func(n int) bool { return n > 10 })
//	// notFound = 0 (zero value), ok = false
//
// Find in struct slices:
//
//	users := []User{{Name: "Alice"}, {Name: "Bob"}}
//	user, found := slicesx.Find(users, func(u User) bool {
//	    return u.Name == "Alice"
//	})
//
// # Common Use Cases
//
// Proto conversion:
//
//	protoUsers := slicesx.Map(users, UserToProto)
//
// ID extraction:
//
//	userIDs := slicesx.Map(users, func(u User) string { return u.ID })
//
// Filtering by conversion:
//
//	activeUser, found := slicesx.Find(users, func(u User) bool {
//	    return u.Active
//	})
package slicesx
