package instance

import "reflect"

// New returns a new instance of type T.
//
// For pointer types, it allocates memory and returns a pointer to a new instance.
// For value types, it returns the zero value.
//
// This is particularly useful when working with generics where the concrete type
// is not known at compile time, such as proto messages or generic containers.
//
// Example:
//
//	// Pointer type (common for proto messages)
//	user := instance.New[*pb.User]()  // Returns &pb.User{}
//
//	// Value type
//	count := instance.New[int]()      // Returns 0
//	name := instance.New[string]()    // Returns ""
func New[T any]() T {
	var zero T
	tType := reflect.TypeOf(zero)

	// For pointer types (like *pb.User), allocate new instance
	if tType != nil && tType.Kind() == reflect.Pointer {
		return reflect.New(tType.Elem()).Interface().(T)
	}

	// For value types, return zero value
	return zero
}

// Zero returns the zero value for type T.
//
// This is helpful when you need to explicitly return a zero value in generic code,
// though in most cases you can just use `var zero T`.
//
// Example:
//
//	func getOrZero[T any](m map[string]T, key string) T {
//	    if val, ok := m[key]; ok {
//	        return val
//	    }
//	    return instance.Zero[T]()
//	}
func Zero[T any]() T {
	var zero T
	return zero
}
