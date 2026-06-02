package ptr

// To returns the value pointed to by the pointer or its zero value if nil.
//
// This provides safe dereferencing of pointers, returning the zero value
// for the type if the pointer is nil instead of panicking.
//
// Example:
//
//	age := ptr.Of(25)
//	value := ptr.To(age)  // 25
//
//	var nilAge *int
//	value := ptr.To(nilAge)  // 0 (zero value for int)
func To[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// SliceTo returns a slice of values pointed to by the pointers.
//
// Nil pointers in the input slice will be converted to zero values
// in the output slice.
//
// Example:
//
//	ptrs := []*int{ptr.Of(1), nil, ptr.Of(3)}
//	values := ptr.SliceTo(ptrs...)  // []int{1, 0, 3}
func SliceTo[T any](ps ...*T) []T {
	slc := make([]T, len(ps))
	for i := range ps {
		slc[i] = To(ps[i])
	}
	return slc
}
