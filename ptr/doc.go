// Package ptr provides generic utility functions for working with pointers.
//
// This package simplifies common pointer operations using Go generics,
// particularly useful when working with optional values, protocol buffers,
// and database fields that may be nullable.
//
// # Creating Pointers
//
// Use Of to create a pointer from a value:
//
//	age := ptr.Of(25)          // *int
//	name := ptr.Of("Alice")    // *string
//	active := ptr.Of(true)     // *bool
//
// Create multiple pointers at once:
//
//	numbers := ptr.SliceOf(1, 2, 3)  // []*int
//
// # Dereferencing Pointers
//
// Use To to safely dereference pointers with zero-value fallback:
//
//	var age *int = ptr.Of(25)
//	value := ptr.To(age)  // 25
//
//	var nilAge *int
//	value := ptr.To(nilAge)  // 0 (zero value for int)
//
// Dereference slices of pointers:
//
//	ptrs := []*int{ptr.Of(1), nil, ptr.Of(3)}
//	values := ptr.SliceTo(ptrs...)  // []int{1, 0, 3}
//
// # Common Use Cases
//
// Protocol buffer optional fields:
//
//	user := &pb.User{
//	    Name: ptr.Of("Alice"),
//	    Age:  ptr.Of(30),
//	}
//
// Database nullable fields:
//
//	var deletedAt *time.Time
//	if shouldDelete {
//	    deletedAt = ptr.Of(time.Now())
//	}
//
// Safe dereferencing:
//
//	displayAge := ptr.To(user.Age)  // Safe even if user.Age is nil
//
// # Type Safety
//
// All functions use Go generics for type safety:
//
//	intPtr := ptr.Of(42)      // Type: *int
//	intVal := ptr.To(intPtr)  // Type: int
package ptr
