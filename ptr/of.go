package ptr

// Of returns a pointer to the provided value.
//
// This is useful when you need a pointer to a literal or value,
// such as for optional proto fields or nullable database columns.
//
// Example:
//
//	age := ptr.Of(25)          // *int pointing to 25
//	name := ptr.Of("Alice")    // *string pointing to "Alice"
//	active := ptr.Of(true)     // *bool pointing to true
func Of[T any](v T) *T {
	return &v
}

// SliceOf returns a slice of pointers from the specified values.
//
// This is useful when you need to create a slice of optional values.
//
// Example:
//
//	numbers := ptr.SliceOf(1, 2, 3)  // []*int{&1, &2, &3}
//	names := ptr.SliceOf("Alice", "Bob")  // []*string{&"Alice", &"Bob"}
func SliceOf[T any](vv ...T) []*T {
	slc := make([]*T, len(vv))
	for i := range vv {
		slc[i] = Of(vv[i])
	}
	return slc
}
