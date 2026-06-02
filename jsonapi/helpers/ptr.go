package helpers

// PointerOf returns a pointer to the provided value.
func PointerOf[T any](v T) *T {
	return &v
}

// SliceOfPointers returns a slice of *T from the specified values.
func SliceOfPointers[T any](vv ...T) []*T {
	slc := make([]*T, len(vv))
	for i := range vv {
		slc[i] = PointerOf(vv[i])
	}
	return slc
}
